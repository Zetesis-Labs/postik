package app_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/zetesis-labs/postik/internal/config"
	"github.com/zetesis-labs/postik/internal/testsupport/fakeoidc"
	"github.com/zetesis-labs/postik/internal/testsupport/faketelegram"
)

var (
	bruno       = fakeoidc.Identity{Subject: "bruno-1", Email: "bruno@example.com", Name: "Bruno"}
	grupo       = faketelegram.Chat{ID: -1001, Type: "supergroup", Title: "Grupo Zetesis", Username: "grupozetesis", Photo: []byte("jpeg-bytes"), BotAdmin: true}
	grupoBruno  = faketelegram.Chat{ID: -2002, Type: "supergroup", Title: "Grupo Bruno", BotAdmin: true}
	sinAdmin    = faketelegram.Chat{ID: -3003, Type: "supergroup", Title: "Sin permisos"}
	otroGrupo   = faketelegram.Chat{ID: -4004, Type: "supergroup", Title: "Otro grupo", BotAdmin: true}
	testBotName = "postik_test_bot"
)

func withTelegram() harnessOption {
	return func(c *config.Config) {
		c.Telegram = &config.Telegram{BotToken: faketelegram.Token}
	}
}

// startTelegram is called by newHarness when the configuration asks for Telegram.
func startTelegram(h *harness, cfg *config.Config) {
	h.bot = faketelegram.New(testBotName)
	h.botAPI = httptest.NewServer(h.bot.Handler())
	h.t.Cleanup(h.botAPI.Close)
	cfg.Telegram.APIURL = h.botAPI.URL
}

// stopTelegram makes the Bot API refuse connections from now on.
func (h *harness) stopTelegram() {
	h.botAPI.Close()
}

func (h *harness) member(identity fakeoidc.Identity) requestOption {
	h.t.Helper()
	b := h.newBrowser()
	if location := b.signIn(identity); location != "/launches" {
		h.t.Fatalf("%s landed on %q", identity.Subject, location)
	}
	return withCookie(b.sessionCookie())
}

type channelView struct {
	ID           string
	Provider     string
	Name         string
	Username     string
	Picture      string
	Disabled     bool
	Customer     string
	PostingTimes []int
}

func (h *harness) channels(session requestOption) []channelView {
	h.t.Helper()
	res := h.do(http.MethodGet, "/api/v1/channels", nil, session)
	expectStatus(h.t, res, http.StatusOK)
	var raw []map[string]any
	decodeJSON(h.t, res.body, &raw)
	views := make([]channelView, len(raw))
	for i, c := range raw {
		views[i] = channelView{ID: c["id"].(string), Provider: c["provider"].(string), Name: c["name"].(string), Disabled: c["disabled"].(bool)}
		views[i].Username, _ = c["username"].(string)
		views[i].Picture, _ = c["picture"].(string)
		if customer, ok := c["customer"].(map[string]any); ok {
			views[i].Customer = customer["name"].(string)
		}
		for _, minute := range c["postingTimes"].([]any) {
			views[i].PostingTimes = append(views[i].PostingTimes, int(minute.(float64)))
		}
	}
	return views
}

func (h *harness) openConnection(session requestOption) (code, bot string) {
	h.t.Helper()
	res := h.do(http.MethodPost, "/api/v1/channels/telegram/connections", nil, session)
	expectStatus(h.t, res, http.StatusCreated)
	body := res.json(h.t)
	return body["code"].(string), body["botUsername"].(string)
}

func (h *harness) connectionStatus(session requestOption, code string) (int, string, string) {
	h.t.Helper()
	res := h.do(http.MethodGet, "/api/v1/channels/telegram/connections/"+code, nil, session)
	if res.status != http.StatusOK {
		return res.status, "", ""
	}
	body := res.json(h.t)
	channelID, _ := body["channelId"].(string)
	return res.status, body["status"].(string), channelID
}

// connect runs F6 for chat and returns the channel ID.
func (h *harness) connect(session requestOption, chat faketelegram.Chat) string {
	h.t.Helper()
	code, _ := h.openConnection(session)
	h.bot.SetChat(chat)
	h.bot.Say(chat.ID, "/connect "+code)
	status, state, channelID := h.connectionStatus(session, code)
	if status != http.StatusOK || state != "connected" {
		h.t.Fatalf("connection %s: status %d, state %q", code, status, state)
	}
	return channelID
}

func findChannel(channels []channelView, id string) (channelView, bool) {
	for _, c := range channels {
		if c.ID == id {
			return c, true
		}
	}
	return channelView{}, false
}

// S03.1 Solo se ofrecen los proveedores con credenciales.
func TestOnlyConfiguredProvidersAreOffered(t *testing.T) {
	without := newHarness(t, withOIDC("Fake"))
	session := without.member(ana)
	res := without.do(http.MethodGet, "/api/v1/channels/providers", nil, session)
	expectStatus(t, res, http.StatusOK)
	if strings.TrimSpace(string(res.body)) != "[]" {
		t.Fatalf("providers without credentials = %s", res.body)
	}
	expectStatus(t, without.do(http.MethodPost, "/api/v1/channels/telegram/connections", nil, session), http.StatusNotFound)

	with := newHarness(t, withOIDC("Fake"), withTelegram())
	res = with.do(http.MethodGet, "/api/v1/channels/providers", nil, with.member(ana))
	var providers []map[string]string
	decodeJSON(t, res.body, &providers)
	if len(providers) != 1 || providers[0]["identifier"] != "telegram" || providers[0]["name"] != "Telegram" {
		t.Fatalf("providers = %s", res.body)
	}
}

// S03.2 Conectar Telegram crea el canal con nombre, usuario y foto.
func TestConnectingTelegramCreatesTheChannel(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)

	code, bot := h.openConnection(session)
	if len(code) != 4 || bot != testBotName {
		t.Fatalf("code %q, bot %q", code, bot)
	}
	h.bot.SetChat(grupo)
	commandID := h.bot.Say(grupo.ID, "/connect "+code)
	status, state, channelID := h.connectionStatus(session, code)
	if status != http.StatusOK || state != "connected" || channelID == "" {
		t.Fatalf("status %d, state %q, channel %q", status, state, channelID)
	}

	channel, found := findChannel(h.channels(session), channelID)
	if !found {
		t.Fatal("the channel is not listed")
	}
	if channel.Provider != "telegram" || channel.Name != "Grupo Zetesis" || channel.Username != "grupozetesis" || channel.Disabled {
		t.Errorf("channel = %+v", channel)
	}
	if !reflect.DeepEqual(channel.PostingTimes, []int{120, 400, 700}) {
		t.Errorf("posting times = %v", channel.PostingTimes)
	}
	if !strings.HasPrefix(channel.Picture, "/uploads/avatars/") {
		t.Fatalf("picture = %q", channel.Picture)
	}
	picture := h.do(http.MethodGet, channel.Picture, nil)
	expectStatus(t, picture, http.StatusOK)
	if string(picture.body) != "jpeg-bytes" {
		t.Errorf("picture body = %q", picture.body)
	}
	if deleted := h.bot.Deleted(grupo.ID); !reflect.DeepEqual(deleted, []int64{commandID}) {
		t.Errorf("deleted messages = %v, want the command %d", deleted, commandID)
	}
}

// S03.3 Sin mensaje la conexión sigue pendiente y a los 30 minutos caduca.
func TestConnectionStaysPendingAndExpires(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	code, _ := h.openConnection(session)

	if _, state, _ := h.connectionStatus(session, code); state != "pending" {
		t.Fatalf("state = %q, want pending", state)
	}
	h.clock.Advance(30 * time.Minute)
	h.bot.SetChat(grupo)
	h.bot.Say(grupo.ID, "/connect "+code)
	if _, state, _ := h.connectionStatus(session, code); state != "expired" {
		t.Fatalf("state = %q, want expired", state)
	}
	if channels := h.channels(session); len(channels) != 0 {
		t.Fatalf("an expired code created %+v", channels)
	}
}

// S03.4 Volver a conectar el mismo chat actualiza el canal y conserva sus ajustes.
func TestReconnectingTheSameChatKeepsItsSettings(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	id := h.connect(session, grupo)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+id+"/customer", map[string]string{"name": "Acme"}, session), http.StatusNoContent)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+id+"/posting-times", map[string][]int{"times": {540}}, session), http.StatusNoContent)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+id+"/disabled", map[string]bool{"disabled": true}, session), http.StatusNoContent)

	renamed := grupo
	renamed.Title = "Zetesis Labs"
	again := h.connect(session, renamed)

	channels := h.channels(session)
	if again != id || len(channels) != 1 {
		t.Fatalf("reconnecting created another channel: %s vs %s, %d channels", again, id, len(channels))
	}
	c := channels[0]
	if c.Name != "Zetesis Labs" || c.Customer != "Acme" || !c.Disabled || !reflect.DeepEqual(c.PostingTimes, []int{540}) {
		t.Fatalf("channel after reconnecting = %+v", c)
	}
}

// S03.5 Cada código conecta en su organización y el de otra no se ve.
func TestEachCodeConnectsInItsOwnOrganization(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	anaSession, brunoSession := h.member(ana), h.member(bruno)
	anaCode, _ := h.openConnection(anaSession)
	brunoCode, _ := h.openConnection(brunoSession)
	h.bot.SetChat(grupo)
	h.bot.SetChat(grupoBruno)
	h.bot.Say(grupo.ID, "/connect "+anaCode)
	h.bot.Say(grupoBruno.ID, "/connect "+brunoCode)

	if _, state, _ := h.connectionStatus(anaSession, anaCode); state != "connected" {
		t.Fatalf("Ana's code: %q", state)
	}
	if status, _, _ := h.connectionStatus(anaSession, brunoCode); status != http.StatusNotFound {
		t.Fatalf("Ana sees Bruno's code with status %d", status)
	}
	if _, state, _ := h.connectionStatus(brunoSession, brunoCode); state != "connected" {
		t.Fatalf("Bruno's code: %q", state)
	}
	anaChannels, brunoChannels := h.channels(anaSession), h.channels(brunoSession)
	if len(anaChannels) != 1 || anaChannels[0].Name != "Grupo Zetesis" {
		t.Errorf("Ana's channels = %+v", anaChannels)
	}
	if len(brunoChannels) != 1 || brunoChannels[0].Name != "Grupo Bruno" {
		t.Errorf("Bruno's channels = %+v", brunoChannels)
	}
}

// S03.6 Si el bot no puede borrar el mensaje, la conexión sigue adelante.
func TestConnectingWorksWhenTheBotCannotDelete(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	h.connect(session, sinAdmin)

	if deleted := h.bot.Deleted(sinAdmin.ID); len(deleted) != 0 {
		t.Fatalf("deleted = %v", deleted)
	}
	if channels := h.channels(session); len(channels) != 1 || channels[0].Name != "Sin permisos" {
		t.Fatalf("channels = %+v", channels)
	}
}

// S03.7 Mover a un cliente por nombre lo crea una vez y se puede deshacer.
func TestMovingToACustomerByName(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session, brunoSession := h.member(ana), h.member(bruno)
	first, second := h.connect(session, grupo), h.connect(session, otroGrupo)
	brunoChannel := h.connect(brunoSession, grupoBruno)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+brunoChannel+"/customer", map[string]string{"name": "Ajeno"}, brunoSession), http.StatusNoContent)
	foreignCustomer := h.customers(brunoSession)["Ajeno"]

	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+first+"/customer", map[string]string{"name": " Acme "}, session), http.StatusNoContent)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+second+"/customer", map[string]string{"name": "Acme"}, session), http.StatusNoContent)
	if customers := h.customers(session); len(customers) != 1 || customers["Acme"] == "" {
		t.Fatalf("customers = %v", customers)
	}
	for _, c := range h.channels(session) {
		if c.Customer != "Acme" {
			t.Errorf("%s is in %q", c.Name, c.Customer)
		}
	}

	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+second+"/customer", map[string]any{"customerId": foreignCustomer}, session), http.StatusNotFound)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+first+"/customer", map[string]any{"customerId": nil}, session), http.StatusNoContent)
	if c, _ := findChannel(h.channels(session), first); c.Customer != "" {
		t.Errorf("the first channel is still in %q", c.Customer)
	}
	if c, _ := findChannel(h.channels(session), second); c.Customer != "Acme" {
		t.Errorf("the second channel moved to %q", c.Customer)
	}
}

func (h *harness) customers(session requestOption) map[string]string {
	h.t.Helper()
	res := h.do(http.MethodGet, "/api/v1/customers", nil, session)
	expectStatus(h.t, res, http.StatusOK)
	var list []map[string]string
	decodeJSON(h.t, res.body, &list)
	byName := map[string]string{}
	for _, c := range list {
		byName[c["name"]] = c["id"]
	}
	return byName
}

// S03.8 Las franjas se guardan ordenadas, sin repetir y dentro del día.
func TestPostingTimesAreSortedUniqueAndWithinTheDay(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	id := h.connect(session, grupo)

	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+id+"/posting-times", map[string][]int{"times": {700, 60, 700, 1439}}, session), http.StatusNoContent)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+id+"/posting-times", map[string][]int{"times": {1440}}, session), http.StatusBadRequest)
	if c, _ := findChannel(h.channels(session), id); !reflect.DeepEqual(c.PostingTimes, []int{60, 700, 1439}) {
		t.Fatalf("posting times = %v", c.PostingTimes)
	}
}

// S03.9 Desactivar y activar un canal.
func TestDisablingAndEnablingAChannel(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	id := h.connect(session, grupo)

	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+id+"/disabled", map[string]bool{"disabled": true}, session), http.StatusNoContent)
	if c, _ := findChannel(h.channels(session), id); !c.Disabled {
		t.Fatal("the channel is not disabled")
	}
	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+id+"/disabled", map[string]bool{"disabled": false}, session), http.StatusNoContent)
	if c, _ := findChannel(h.channels(session), id); c.Disabled {
		t.Fatal("the channel is still disabled")
	}
}

// S03.10 Borrar un canal lo quita de la lista.
func TestDeletingAChannel(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	gone, kept := h.connect(session, grupo), h.connect(session, otroGrupo)

	expectStatus(t, h.do(http.MethodDelete, "/api/v1/channels/"+gone, nil, session), http.StatusNoContent)
	channels := h.channels(session)
	if len(channels) != 1 || channels[0].ID != kept {
		t.Fatalf("channels = %+v", channels)
	}
}

// S03.11 Los canales de otra organización no se ven ni se tocan.
func TestChannelsOfAnotherOrganizationAreOutOfReach(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	anaSession, brunoSession := h.member(ana), h.member(bruno)
	brunoChannel := h.connect(brunoSession, grupoBruno)

	if channels := h.channels(anaSession); len(channels) != 0 {
		t.Fatalf("Ana sees %+v", channels)
	}
	attempts := []struct {
		method, path string
		body         any
	}{
		{http.MethodPut, "/disabled", map[string]bool{"disabled": true}},
		{http.MethodPut, "/customer", map[string]string{"name": "Robado"}},
		{http.MethodPut, "/posting-times", map[string][]int{"times": {1}}},
		{http.MethodDelete, "", nil},
	}
	for _, a := range attempts {
		res := h.do(a.method, "/api/v1/channels/"+brunoChannel+a.path, a.body, anaSession)
		expectStatus(t, res, http.StatusNotFound)
	}
	channels := h.channels(brunoSession)
	if len(channels) != 1 || channels[0].Disabled || channels[0].Customer != "" || !reflect.DeepEqual(channels[0].PostingTimes, []int{120, 400, 700}) {
		t.Fatalf("Bruno's channel changed: %+v", channels)
	}
	if customers := h.customers(anaSession); len(customers) != 0 {
		t.Fatalf("Ana got customers %v", customers)
	}
}
