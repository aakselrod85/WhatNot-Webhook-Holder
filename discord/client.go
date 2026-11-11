package discord

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	apiBase        = "https://discord.com/api/v10"
	maxRateRetries = 3
)

// ErrMessageGone is returned by EditMessage when Discord answers 404 (the message was deleted).
var ErrMessageGone = errors.New("discord message not found")

type Client struct {
	token string
	http  *http.Client
}

func New(token string) *Client {
	return &Client{token: token, http: &http.Client{Timeout: 15 * time.Second}}
}

// Enabled is false without a bot token: the whole integration is then a no-op.
func (c *Client) Enabled() bool {
	return c != nil && c.token != ""
}

func (c *Client) CreateMessage(channelId string, payload Message) (string, error) {
	status, body, err := c.do(http.MethodPost, "/channels/"+channelId+"/messages", payload)
	if err != nil {
		return "", err
	}
	if status < 200 || status > 299 {
		return "", fmt.Errorf("discord create message: status %d: %s", status, string(body))
	}
	var resp struct {
		Id string `json:"id"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || resp.Id == "" {
		return "", fmt.Errorf("discord create message: no id in response")
	}
	return resp.Id, nil
}

func (c *Client) EditMessage(channelId, messageId string, payload Message) error {
	status, body, err := c.do(http.MethodPatch, "/channels/"+channelId+"/messages/"+messageId, payload)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		return ErrMessageGone
	}
	if status < 200 || status > 299 {
		return fmt.Errorf("discord edit message: status %d: %s", status, string(body))
	}
	return nil
}

// DeleteMessage treats 404 as success.
func (c *Client) DeleteMessage(channelId, messageId string) error {
	status, body, err := c.do(http.MethodDelete, "/channels/"+channelId+"/messages/"+messageId, nil)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound || (status >= 200 && status <= 299) {
		return nil
	}
	return fmt.Errorf("discord delete message: status %d: %s", status, string(body))
}

// PinMessage pins the message; falls back once to the legacy pins endpoint when the current one answers 404/405.
func (c *Client) PinMessage(channelId, messageId string) error {
	status, body, err := c.do(http.MethodPut, "/channels/"+channelId+"/messages/pins/"+messageId, nil)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		status, body, err = c.do(http.MethodPut, "/channels/"+channelId+"/pins/"+messageId, nil)
		if err != nil {
			return err
		}
	}
	if status < 200 || status > 299 {
		return fmt.Errorf("discord pin message: status %d: %s", status, string(body))
	}
	return nil
}

// UnpinMessage removes the pin; a message or pin that no longer exists counts as success.
func (c *Client) UnpinMessage(channelId, messageId string) error {
	status, body, err := c.do(http.MethodDelete, "/channels/"+channelId+"/messages/pins/"+messageId, nil)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		status, body, err = c.do(http.MethodDelete, "/channels/"+channelId+"/pins/"+messageId, nil)
		if err != nil {
			return err
		}
		if status == http.StatusNotFound {
			return nil
		}
	}
	if status < 200 || status > 299 {
		return fmt.Errorf("discord unpin message: status %d: %s", status, string(body))
	}
	return nil
}

// DeletePinNotice deletes the "pinned a message" system message (type 6) that refers to messageId, if it is among
// the latest messages. Not finding it is not an error.
func (c *Client) DeletePinNotice(channelId, messageId string) error {
	time.Sleep(500 * time.Millisecond)
	status, body, err := c.do(http.MethodGet, "/channels/"+channelId+"/messages?limit=10", nil)
	if err != nil {
		return err
	}
	if status < 200 || status > 299 {
		return fmt.Errorf("discord list messages: status %d: %s", status, string(body))
	}
	var msgs []struct {
		Id               string `json:"id"`
		Type             int    `json:"type"`
		MessageReference *struct {
			MessageId string `json:"message_id"`
		} `json:"message_reference"`
	}
	if err := json.Unmarshal(body, &msgs); err != nil {
		return err
	}
	for _, m := range msgs {
		if m.Type == 6 && m.MessageReference != nil && m.MessageReference.MessageId == messageId {
			return c.DeleteMessage(channelId, m.Id)
		}
	}
	return nil
}

// ListGuildIds returns the ids of the servers the bot is in.
func (c *Client) ListGuildIds() ([]string, error) {
	status, body, err := c.do(http.MethodGet, "/users/@me/guilds", nil)
	if err != nil {
		return nil, err
	}
	if status < 200 || status > 299 {
		return nil, fmt.Errorf("discord list guilds: status %d: %s", status, string(body))
	}
	var guilds []struct {
		Id string `json:"id"`
	}
	if err := json.Unmarshal(body, &guilds); err != nil {
		return nil, err
	}
	ids := make([]string, len(guilds))
	for i, g := range guilds {
		ids[i] = g.Id
	}
	return ids, nil
}

// SetGuildCommands replaces the application's commands in one server with the given list.
func (c *Client) SetGuildCommands(applicationId, guildId string, commands any) error {
	status, body, err := c.do(http.MethodPut, "/applications/"+applicationId+"/guilds/"+guildId+"/commands", commands)
	if err != nil {
		return err
	}
	if status < 200 || status > 299 {
		return fmt.Errorf("discord set guild commands: status %d: %s", status, string(body))
	}
	return nil
}

// EditInteractionResponse replaces the deferred reply of an interaction. The token authorizes it, no bot header.
func (c *Client) EditInteractionResponse(applicationId, token string, payload Message) error {
	status, body, err := c.doReq(http.MethodPatch, "/webhooks/"+applicationId+"/"+token+"/messages/@original", payload, false)
	if err != nil {
		return err
	}
	if status < 200 || status > 299 {
		return fmt.Errorf("discord edit interaction response: status %d: %s", status, string(body))
	}
	return nil
}

// EditInteractionComponents replaces only the components of the interaction's message (embeds stay as they are).
func (c *Client) EditInteractionComponents(applicationId, token string, components any) error {
	payload := map[string]any{"components": components}
	status, body, err := c.doReq(http.MethodPatch, "/webhooks/"+applicationId+"/"+token+"/messages/@original", payload, false)
	if err != nil {
		return err
	}
	if status < 200 || status > 299 {
		return fmt.Errorf("discord edit interaction components: status %d: %s", status, string(body))
	}
	return nil
}

func (c *Client) do(method, path string, payload any) (int, []byte, error) {
	return c.doReq(method, path, payload, true)
}

// doReq sends the request and, on 429, waits retry_after and retries up to maxRateRetries times.
func (c *Client) doReq(method, path string, payload any, bot bool) (int, []byte, error) {
	var raw []byte
	if payload != nil {
		var err error
		raw, err = json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequest(method, apiBase+path, bytes.NewReader(raw))
		if err != nil {
			return 0, nil, err
		}
		if bot {
			req.Header.Set("Authorization", "Bot "+c.token)
		}
		if raw != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return 0, nil, err
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if err != nil {
			return 0, nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests || attempt >= maxRateRetries {
			return resp.StatusCode, body, nil
		}
		time.Sleep(retryAfter(resp, body))
	}
}

func retryAfter(resp *http.Response, body []byte) time.Duration {
	var rl struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if json.Unmarshal(body, &rl) == nil && rl.RetryAfter > 0 {
		return time.Duration(rl.RetryAfter * float64(time.Second))
	}
	if v, err := strconv.ParseFloat(resp.Header.Get("Retry-After"), 64); err == nil && v > 0 {
		return time.Duration(v * float64(time.Second))
	}
	return time.Second
}
