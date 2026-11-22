package sub

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

func profileTestDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if db := database.GetDB(); db != nil {
			if sqlDB, err := db.DB(); err == nil {
				sqlDB.Close()
			}
		}
	})
}

func profileTestTags(t *testing.T, config string) map[string]bool {
	t.Helper()
	var parsed struct {
		Outbounds []map[string]interface{} `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		t.Fatal(err)
	}
	tags := make(map[string]bool, len(parsed.Outbounds))
	for _, outbound := range parsed.Outbounds {
		tag, _ := outbound["tag"].(string)
		tags[tag] = true
	}
	return tags
}

// A per-inbound profile is the sing-box app's unit of import: one remote
// profile per node. It must carry that inbound's outbound and nothing else,
// while the unfiltered config keeps every node and the external links.
func TestGetJsonPerInboundProfile(t *testing.T) {
	profileTestDB(t)
	db := database.GetDB()

	inbounds := []model.Inbound{
		{
			Type:   "vless",
			Tag:    "one",
			Enable: true,
			OutJson: json.RawMessage(`{"type":"vless","tag":"one",
				"server":"one.example.com","server_port":443}`),
			Addrs: json.RawMessage(`[]`),
		},
		{
			Type:   "vless",
			Tag:    "two",
			Enable: true,
			OutJson: json.RawMessage(`{"type":"vless","tag":"two",
				"server":"two.example.com","server_port":443}`),
			Addrs: json.RawMessage(`[]`),
		},
	}
	if err := db.Create(&inbounds).Error; err != nil {
		t.Fatal(err)
	}

	client := model.Client{
		Name:   "alice",
		Enable: true,
		Config: json.RawMessage(`{"vless":{
			"name":"alice","uuid":"00000000-0000-0000-0000-000000000000"}}`),
		Inbounds: json.RawMessage(fmt.Sprintf(`[%d,%d]`, inbounds[0].Id, inbounds[1].Id)),
		Links:    json.RawMessage(`[{"type":"external","remark":"ext","uri":"socks://127.0.0.1:1080#ext"}]`),
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}

	one, _, err := (&JsonService{}).GetJson("alice", inbounds[0].Id)
	if err != nil {
		t.Fatal(err)
	}
	oneTags := profileTestTags(t, *one)
	if !oneTags["one"] {
		t.Errorf("per-inbound profile lost its own outbound: %v", oneTags)
	}
	if oneTags["two"] {
		t.Errorf("per-inbound profile leaked another inbound: %v", oneTags)
	}
	if oneTags["ext"] {
		t.Errorf("per-inbound profile carried an external link: %v", oneTags)
	}

	all, _, err := (&JsonService{}).GetJson("alice", 0)
	if err != nil {
		t.Fatal(err)
	}
	allTags := profileTestTags(t, *all)
	for _, tag := range []string{"one", "two", "ext"} {
		if !allTags[tag] {
			t.Errorf("the all-inbounds profile lost %q: %v", tag, allTags)
		}
	}
}

// An inbound the client does not own must fail the request rather than serve
// an empty profile that looks like a successful import.
func TestGetJsonRejectsForeignInbound(t *testing.T) {
	profileTestDB(t)
	db := database.GetDB()

	inbound := model.Inbound{
		Type:    "vless",
		Tag:     "mine",
		Enable:  true,
		OutJson: json.RawMessage(`{"type":"vless","tag":"mine","server":"a.example.com","server_port":443}`),
		Addrs:   json.RawMessage(`[]`),
	}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	other := model.Inbound{
		Type:    "vless",
		Tag:     "other",
		Enable:  true,
		OutJson: json.RawMessage(`{"type":"vless","tag":"other","server":"b.example.com","server_port":443}`),
		Addrs:   json.RawMessage(`[]`),
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}

	client := model.Client{
		Name:     "alice",
		Enable:   true,
		Config:   json.RawMessage(`{"vless":{"name":"alice","uuid":"00000000-0000-0000-0000-000000000000"}}`),
		Inbounds: json.RawMessage(fmt.Sprintf(`[%d]`, inbound.Id)),
		Links:    json.RawMessage(`[]`),
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}

	if _, _, err := (&JsonService{}).GetJson("alice", other.Id); err == nil {
		t.Fatal("an inbound the client does not own must be rejected")
	}
}
