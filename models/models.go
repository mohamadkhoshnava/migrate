package models

import (
	"log"
	"remnawave-migrate/util"
	"strings"
	"time"
)

type UsersResponse struct {
	Users []User `json:"users"`
	Total int    `json:"total"`
}

type User struct {
	MarzbanUser   MarzbanUser
	ProcessedUser ProcessedUser
}

func (u *User) Process() ProcessedUser {
	if u.ProcessedUser.Username != "" {
		return u.ProcessedUser
	}

	return u.MarzbanUser.Process()
}

type MarzbanProxies struct {
	Vless struct {
		ID   string `json:"id"`
		Flow string `json:"flow"`
	} `json:"vless"`
	Trojan struct {
		Password string `json:"password"`
		Flow     string `json:"flow"`
	} `json:"trojan"`
	Shadowsocks struct {
		Password string `json:"password"`
		Method   string `json:"method"`
	} `json:"shadowsocks"`
}

type MarzbanUser struct {
	Proxies                MarzbanProxies      `json:"proxies"`
	CreatedAt              string              `json:"created_at"`
	Expire                 int64               `json:"expire"`
	DataLimit              int64               `json:"data_limit"`
	UsedTraffic            int64               `json:"used_traffic"`
	DataLimitResetStrategy string              `json:"data_limit_reset_strategy"`
	Inbounds               map[string][]string `json:"inbounds"`
	Note                   string              `json:"note"`
	Username               string              `json:"username"`
	Status                 string              `json:"status"`
	SubscriptionURL        string              `json:"subscription_url"`
}

type MarzbanUsersResponse struct {
	Users []MarzbanUser `json:"users"`
	Total int           `json:"total"`
}

type ProcessedUser struct {
	CreatedAt              string   `json:"created_at"`
	Expire                 string   `json:"expire"`
	DataLimit              int64    `json:"data_limit"`
	RemainingVolume        int64    `json:"remaining_volume"`
	DataLimitResetStrategy string   `json:"data_limit_reset_strategy"`
	InboundTags            []string `json:"inbounds"`
	Note                   string   `json:"note"`
	Username               string   `json:"username"`
	Status                 string   `json:"status"`
	VlessID                string   `json:"vless_id"`
	TrojanPassword         string   `json:"trojan_password"`
	ShadowsocksPassword    string   `json:"shadowsocks_password"`
	SubscriptionHash       string   `json:"subscription_hash"`
}

func (u *MarzbanUser) Process() ProcessedUser {
	var expireTime time.Time
	if u.Expire > 0 {
		expireTime = time.Unix(u.Expire, 0).UTC()
	} else {
		expireTime = time.Date(2099, 12, 31, 15, 13, 22, 214000000, time.UTC)
	}

	subscriptionHash := ""
	if u.SubscriptionURL != "" {
		parts := strings.Split(u.SubscriptionURL, "/")
		if len(parts) > 0 {
			subscriptionHash = parts[len(parts)-1]
		}
	}

	// combine all inbounds into one list
	var inboundTags []string
	for _, values := range u.Inbounds {
		inboundTags = append(inboundTags, values...)
	}

	parsedCreatedAt, err := time.Parse("2006-01-02T15:04:05", u.CreatedAt)
	if err != nil {
		parsedCreatedAt = time.Now().UTC()
	}

	// Calculate remaining volume: data_limit - used_traffic
	remainingVolume := u.DataLimit - u.UsedTraffic
	if remainingVolume < 0 {
		remainingVolume = 0
	}

	return ProcessedUser{
		CreatedAt:              parsedCreatedAt.Format("2006-01-02T15:04:05.000Z"),
		Expire:                 expireTime.Format("2006-01-02T15:04:05.000Z"),
		DataLimit:              u.DataLimit,
		RemainingVolume:        remainingVolume,
		DataLimitResetStrategy: u.DataLimitResetStrategy,
		Note:                   u.Note,
		InboundTags:            inboundTags,
		Username:               u.Username,
		Status:                 u.Status,
		VlessID:                u.Proxies.Vless.ID,
		TrojanPassword:         u.Proxies.Trojan.Password,
		ShadowsocksPassword:    u.Proxies.Shadowsocks.Password,
		SubscriptionHash:       subscriptionHash,
	}
}

type CreateUserRequest struct {
	Username             string   `json:"username"`
	Status               string   `json:"status"`
	ShortUUID            *string  `json:"shortUuid,omitempty"`
	TrojanPassword       *string  `json:"trojanPassword,omitempty"`
	VlessUUID            *string  `json:"vlessUuid,omitempty"`
	SsPassword           *string  `json:"ssPassword,omitempty"`
	TrafficLimitBytes    int64    `json:"trafficLimitBytes"`
	TrafficLimitStrategy string   `json:"trafficLimitStrategy"`
	ActiveUserInbounds   []string `json:"activeUserInbounds"`
	ExpireAt             string   `json:"expireAt"`
	CreatedAt            string   `json:"createdAt"`
	Description          string   `json:"description"`
	ActivateAllInbounds  bool     `json:"activateAllInbounds"`
}

func (p *ProcessedUser) ToCreateUserRequest(preferredStrategy string, preserveStatus bool, preserveSubHash bool, preserveInbounds bool, remnawaveInbounds map[string]string) CreateUserRequest {
	strategy := strings.ToUpper(p.DataLimitResetStrategy)

	if strategy == "YEAR" {
		strategy = "NO_RESET"
	}

	if preferredStrategy != "" {
		strategy = preferredStrategy
	}

	status := "ACTIVE"
	if preserveStatus && strings.ToLower(p.Status) != "on_hold" {
		status = strings.ToUpper(p.Status)
	}

	validUsername := util.SanitizeUsername(p.Username)

	req := CreateUserRequest{
		Username:             validUsername,
		Status:               status,
		TrafficLimitBytes:    p.RemainingVolume,
		TrafficLimitStrategy: strategy,
		ActiveUserInbounds:   []string{},
		ExpireAt:             p.Expire,
		CreatedAt:            p.CreatedAt,
		Description:          p.Note,
		ActivateAllInbounds:  true,
	}

	if preserveInbounds {
		var inboundUuidList []string
		for _, tag := range p.InboundTags {
			if uuid, ok := remnawaveInbounds[tag]; ok {
				inboundUuidList = append(inboundUuidList, uuid)
			} else {
				log.Printf("Warning: inbound tag %s not found in destination panel, skipping", tag)
			}
		}

		req.ActiveUserInbounds = inboundUuidList
		req.ActivateAllInbounds = false
	}

	if preserveSubHash && p.SubscriptionHash != "" {
		req.ShortUUID = strPtr(p.SubscriptionHash)
	}
	if p.TrojanPassword != "" {
		req.TrojanPassword = strPtr(p.TrojanPassword)
	}
	if p.VlessID != "" {
		req.VlessUUID = strPtr(p.VlessID)
	}
	if p.ShadowsocksPassword != "" {
		req.SsPassword = strPtr(p.ShadowsocksPassword)
	}

	return req
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
