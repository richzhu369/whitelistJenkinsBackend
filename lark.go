package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const webhookURL = "https://open.larksuite.com/open-apis/bot/v2/hook/2ee21888-8088-463d-ad32-d8ab70c09696"

type LarkMessage struct {
	MsgType string `json:"msg_type"`
	Content struct {
		Text string `json:"text"`
	} `json:"content"`
}

func SendToLark(message string) error {
	msg := LarkMessage{
		MsgType: "text",
	}
	msg.Content.Text = message

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send message to Lark, status code: %d", resp.StatusCode)
	}

	return nil
}
