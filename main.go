package main

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	Token       string `json:"token"`
	IDToken     string `json:"id_token"`
}

func main() {
	log.SetLevel(log.TraceLevel)
	log.SetFormatter(&log.JSONFormatter{
		DisableHTMLEscape: true,
	})
	tokenUrl := os.Getenv("TOKEN_URL")
	clientId := os.Getenv("CLIENT_ID")

	m := http.NewServeMux()
	m.HandleFunc("/token", handler(tokenUrl, clientId))
	m.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	})
	log.Debug("listening on :9001")
	http.ListenAndServe("0.0.0.0:9001", m)
}

func traceRequest(r *http.Request) *http.Request {
	var start, connect, dns, tlsHandshake time.Time
	l := log.WithField("component", "upstream_request")
	trace := &httptrace.ClientTrace{
		DNSStart: func(dsi httptrace.DNSStartInfo) { dns = time.Now() },
		DNSDone: func(ddi httptrace.DNSDoneInfo) {
			l.WithField("time_ms", time.Since(dns).Milliseconds()).Trace("DNS Done")
		},
		TLSHandshakeStart: func() { tlsHandshake = time.Now() },
		TLSHandshakeDone: func(cs tls.ConnectionState, err error) {
			l.WithField("time_ms", time.Since(tlsHandshake).Milliseconds()).Trace("TLS Done")
		},
		ConnectStart: func(network, addr string) { connect = time.Now() },
		ConnectDone: func(network, addr string, err error) {
			l.WithField("time_ms", time.Since(connect).Milliseconds()).Trace("Connect time")
		},
		GotFirstResponseByte: func() {
			l.WithField("time_ms", time.Since(start).Milliseconds()).Trace("Time to first byte")
		},
	}
	start = time.Now()
	return r.WithContext(httptrace.WithClientTrace(r.Context(), trace))
}

func handler(tokenUrl string, clientId string) func(http.ResponseWriter, *http.Request) {
	specialScope := os.Getenv("SCOPE")
	return func(w http.ResponseWriter, r *http.Request) {
		service := r.URL.Query().Get("service")
		scope := r.URL.Query().Get("scope")
		offline := r.URL.Query().Get("offline_token")

		user, password, ok := r.BasicAuth()
		if !ok {
			http.Error(w, "Authorization required", http.StatusUnauthorized)
			return
		}

		log.WithFields(log.Fields{
			"service": service,
			"scope":   scope,
			"user":    user,
			"remote":  r.Header.Get("X-Forwarded-For"),
		}).Info("token request")

		data := url.Values{
			"client_id":  []string{clientId},
			"grant_type": []string{"client_credentials"},
			"username":   []string{user},
			"password":   []string{password},
			"scope":      []string{scope + " " + specialScope},
		}
		req, err := http.NewRequest("POST", tokenUrl, strings.NewReader(data.Encode()))
		if err != nil {
			log.WithError(err).Warning("failed to create token request")
			return
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))
		req = traceRequest(req)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.WithError(err).Warning("failed to send token request")
			return
		}
		var tr TokenResponse
		err = json.NewDecoder(res.Body).Decode(&tr)
		if err != nil {
			log.WithError(err).Warning("failed to parse token response")
		}
		if offline == "true" {
			err = json.NewEncoder(w).Encode(TokenResponse{
				AccessToken: tr.AccessToken,
				IDToken:     tr.IDToken,
			})
		} else {
			err = json.NewEncoder(w).Encode(TokenResponse{
				Token:   tr.AccessToken,
				IDToken: tr.IDToken,
			})
		}
		if err != nil {
			log.WithError(err).Warning("failed to write response")
		}
	}
}
