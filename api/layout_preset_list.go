package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/SaloEater/WhatNot-Webhook-Holder/service"
)

func (a *API) ListLayoutPresets(w http.ResponseWriter, r *http.Request) (any, error) {
	request := service.ListLayoutPresetsRequest{}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Println("An error occurred during reading body of list layout presets: " + err.Error())
		return nil, err
	}
	err = json.Unmarshal(body, &request)
	if err != nil {
		fmt.Println("An error occurred during unmarshalling body of list layout presets: " + err.Error())
		return nil, err
	}
	return a.Service.ListLayoutPresets(&request)
}
