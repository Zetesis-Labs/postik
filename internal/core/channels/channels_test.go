package channels

import (
	"reflect"
	"testing"
	"time"
)

func TestParseConnectCommand(t *testing.T) {
	cases := map[string]string{
		"/connect aB3x":                 "aB3x",
		"  /connect aB3x  ":             "aB3x",
		"/connect@postik_test_bot aB3x": "aB3x",
		"/connect@POSTIK_TEST_BOT aB3x": "aB3x",
		"/connect@otro_bot aB3x":        "",
		"/connect abc":                  "",
		"/connect abcde":                "",
		"/connect ab-d":                 "",
		"hola /connect aB3x":            "",
		"/connectaB3x":                  "",
	}
	for text, want := range cases {
		got, ok := ParseConnectCommand(text, "postik_test_bot")
		if got != want || ok != (want != "") {
			t.Errorf("ParseConnectCommand(%q) = %q, %v; want %q", text, got, ok, want)
		}
	}
}

func TestConnectionState(t *testing.T) {
	created := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	if got := ConnectionState(created, false, created.Add(29*time.Minute)); got != Pending {
		t.Errorf("at 29 min: %v", got)
	}
	if got := ConnectionState(created, false, created.Add(30*time.Minute)); got != Expired {
		t.Errorf("at 30 min: %v", got)
	}
	if got := ConnectionState(created, true, created.Add(time.Hour)); got != Connected {
		t.Errorf("connected stays connected: %v", got)
	}
}

func TestChatName(t *testing.T) {
	if got := ChatName(Chat{Type: "supergroup", Title: "Grupo Zetesis"}); got != "Grupo Zetesis" {
		t.Errorf("group: %q", got)
	}
	if got := ChatName(Chat{Type: "private", FirstName: "Ana", LastName: "Pérez"}); got != "Ana Pérez" {
		t.Errorf("private: %q", got)
	}
	if got := ChatName(Chat{Type: "private", FirstName: "Ana"}); got != "Ana" {
		t.Errorf("private without last name: %q", got)
	}
}

func TestNormalizePostingTimes(t *testing.T) {
	got, err := NormalizePostingTimes([]int{700, 60, 700, 1439})
	if err != nil || !reflect.DeepEqual(got, []int{60, 700, 1439}) {
		t.Fatalf("got %v, %v", got, err)
	}
	for _, bad := range [][]int{{1440}, {-1}} {
		if _, err := NormalizePostingTimes(bad); err == nil {
			t.Errorf("%v was accepted", bad)
		}
	}
	if got, err := NormalizePostingTimes(nil); err != nil || len(got) != 0 {
		t.Errorf("no slots: %v, %v", got, err)
	}
}

func TestCustomerName(t *testing.T) {
	if got := CustomerName("  Acme  "); got != "Acme" {
		t.Errorf("got %q", got)
	}
	if got := CustomerName("   "); got != "" {
		t.Errorf("blank: %q", got)
	}
}
