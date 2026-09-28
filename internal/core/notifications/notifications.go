// Package notifications composes what postik tells an organization about its
// publications, for the emails (§8). The screens compose their own text from
// the same template and parameters, in their language.
package notifications

import (
	"fmt"
	"html"
	"strings"
)

// Notice is a notification: Kind is info, failure or success.
type Notice struct {
	Kind     string
	Template string
	Params   map[string]string
}

var reasons = map[string]string{
	"channel_disabled": "el canal está desactivado",
	"channel_refresh":  "hay que volver a conectar el canal",
	"unconfirmed":      "no se ha podido confirmar la publicación",
	"unreachable":      "no se ha podido conectar con la red",
}

func provider(n Notice) string {
	name := n.Params["provider"]
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func reason(n Notice) string {
	if r, ok := reasons[n.Params["reason"]]; ok {
		return r
	}
	return n.Params["reason"]
}

// Text is the sentence of a notification, in Spanish.
func Text(n Notice) string {
	channel := n.Params["channel"]
	switch n.Template {
	case "published":
		return fmt.Sprintf("Tu post se ha publicado en %s: %s", provider(n), n.Params["url"])
	case "failed":
		return fmt.Sprintf("Error al publicar en %s en %s: %s", provider(n), channel, reason(n))
	case "comment_failed":
		return fmt.Sprintf("Error al publicar los comentarios en %s en %s: %s", provider(n), channel, reason(n))
	case "unconfirmed":
		return fmt.Sprintf("No se ha podido confirmar tu post en %s. Revisa %s", provider(n), channel)
	case "channel_disabled":
		return fmt.Sprintf("No se ha podido publicar en %s porque está desactivado. Actívalo y vuelve a intentarlo", channel)
	case "channel_refresh":
		return fmt.Sprintf("No se ha podido publicar en %s porque hay que volver a conectarlo", channel)
	}
	return n.Template
}

func subject(n Notice) string {
	channel := n.Params["channel"]
	switch n.Template {
	case "published":
		return "Tu post se ha publicado en " + provider(n)
	case "failed", "comment_failed":
		return fmt.Sprintf("Error al publicar en %s en %s", provider(n), channel)
	case "unconfirmed":
		return "No se ha podido confirmar tu post en " + provider(n)
	}
	return "No se ha podido publicar en " + channel
}

// Email is the subject and HTML body of a notification sent on its own.
func Email(n Notice, publicURL string) (string, string) {
	body := "<p>" + html.EscapeString(Text(n)) + "</p>" +
		`<p><a href="` + html.EscapeString(publicURL) + `/launches">Abrir el calendario</a></p>`
	return subject(n), layout(body, publicURL)
}

// Digest is the hourly summary of the publications of an organization.
func Digest(items []Notice, publicURL string) (string, string) {
	var b strings.Builder
	b.WriteString("<p>Estos posts se han publicado en la última hora:</p><ul>")
	for _, n := range items {
		url := html.EscapeString(n.Params["url"])
		fmt.Fprintf(&b, `<li>%s: <a href="%s">%s</a></li>`, html.EscapeString(provider(n)), url, url)
	}
	b.WriteString("</ul>")
	return fmt.Sprintf("%d posts publicados en la última hora", len(items)), layout(b.String(), publicURL)
}

func layout(body, publicURL string) string {
	settings := html.EscapeString(publicURL) + "/settings"
	return `<div style="font-family:sans-serif">` + body +
		`<hr><p style="font-size:12px;color:#666">Puedes elegir qué correos recibes en <a href="` + settings + `">los ajustes de postik</a>.</p></div>`
}
