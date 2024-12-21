package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
)

const webhookURL = "https://open.larksuite.com/open-apis/bot/v2/hook/2ee21888-8088-463d-ad32-d8ab70c09696"

type LarkMessage struct {
	MsgType string `json:"msg_type"`
	Content struct {
		Text string `json:"text"`
	} `json:"content"`
}

func SendToLark(message string) {
	msg := LarkMessage{
		MsgType: "text",
	}
	msg.Content.Text = message

	data, err := json.Marshal(msg)
	if err != nil {
		log.Println("序列化消息失败:", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Println("发送消息到lark失败:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("发送消息到lark失败, 错误代码:", resp.StatusCode)
	}
}
