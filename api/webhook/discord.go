package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/SaloEater/WhatNot-Webhook-Holder/discord"
	"github.com/SaloEater/WhatNot-Webhook-Holder/service"
)

// DiscordInteractions is Discord's Interactions Endpoint. Registered as "POST /discord/interactions";
// deliberately not wrapped by WrapRoute, because the signature covers the raw body.
func DiscordInteractions(svc *service.Service, publicKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if publicKey == "" || !discord.VerifyInteraction(publicKey, r.Header.Get("X-Signature-Ed25519"), r.Header.Get("X-Signature-Timestamp"), body) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var interaction discord.Interaction
		if err := json.Unmarshal(body, &interaction); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// acked is closed once the response is written: the background work edits the message only after that.
		acked := make(chan struct{})
		defer close(acked)

		responseType := discord.ResponseTypeDeferredUpdateMsg
		switch interaction.Type {
		case discord.InteractionTypePing:
			responseType = discord.ResponseTypePong
		case discord.InteractionTypeMessageComponent:
			// An accepted click answers with the same message, all its controls disabled, until the new page is ready.
			if loading, accepted := svc.HandleDiscordComponent(interaction, acked); accepted && loading != nil {
				resp, err := json.Marshal(map[string]any{
					"type": discord.ResponseTypeUpdateMessage,
					"data": map[string]any{"components": loading},
				})
				if err == nil {
					w.Header().Set("Content-Type", "application/json")
					if _, err := w.Write(resp); err != nil {
						fmt.Println("discord interaction: write response: " + err.Error())
					}
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
					return
				}
			}
		case discord.InteractionTypeCommand:
			svc.HandleDiscordCommand(interaction)
			// Deferred public reply ("bot is thinking..."); the command goroutine replaces it with the result.
			responseType = discord.ResponseTypeDeferredMessage
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprintf(w, `{"type":%d}`, responseType); err != nil {
			fmt.Println("discord interaction: write response: " + err.Error())
		}
	}
}
