package internal

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	Token       string `json:"token"`
	IDToken     string `json:"id_token"`
}

type Server struct {
	clientId string
	tokenUrl string
	scope    string

	anonUsername    string
	anonPassword    string
	anonKubeJWT     bool
	passJWTUsername string

	kubeJWT string

	m *mux.Router
	l *log.Entry
}

func New() *Server {
	m := mux.NewRouter()

	s := &Server{
		clientId:        os.Getenv("CLIENT_ID"),
		tokenUrl:        os.Getenv("TOKEN_URL"),
		scope:           os.Getenv("SCOPE"),
		anonUsername:    os.Getenv("ANON_USERNAME"),
		anonPassword:    os.Getenv("ANON_PASSWORD"),
		anonKubeJWT:     os.Getenv("ANON_KUBE_JWT") != "",
		passJWTUsername: os.Getenv("PASS_JWT_USERNAME"),
		m:               m,
		l:               log.WithField("component", "server"),
	}

	if s.anonKubeJWT {
		s.kubeJWT = s.getKubeJWT()
	}
	sm := m.NewRoute().Subrouter()
	sm.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	})

	m.Use(NewLoggingHandler(s.l, nil))
	m.HandleFunc("/token", s.handler)
	return s
}

func (s *Server) Run() {
	s.l.Info("listening on :9001")
	http.ListenAndServe("0.0.0.0:9001", s.m)
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

func (s *Server) getKubeJWT() string {
	f, err := os.Open("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		s.l.WithError(err).Warning("failed to get kube jwt")
		return ""
	}
	defer f.Close()
	body, err := ioutil.ReadAll(f)
	if err != nil {
		s.l.WithError(err).Warning("failed to read kube jwt")
		return ""
	}
	return string(body)
}

func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	scope := r.URL.Query().Get("scope")
	offline := r.URL.Query().Get("offline_token")

	user, password, ok := r.BasicAuth()
	log.WithField("user", user).WithField("password", password).Trace("tracing credentials")
	data := url.Values{
		"client_id":  []string{s.clientId},
		"grant_type": []string{"client_credentials"},
		"username":   []string{user},
		"password":   []string{password},
		"scope":      []string{scope + " " + s.scope},
	}
	if !ok {
		if s.anonKubeJWT {
			data["client_assertion_type"] = []string{"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"}
			data["client_assertion"] = []string{s.kubeJWT}
			delete(data, "username")
			delete(data, "password")
		} else if s.anonUsername != "" && s.anonPassword != "" {
			user = s.anonUsername
			password = s.anonPassword
		} else {
			w.Header().Set("WWW-Authenticate", `Basic realm="distribution-oauth", charset="UTF-8"`)
			http.Error(w, "Authorization required", http.StatusUnauthorized)
			return
		}
	}
	if s.passJWTUsername != "" && user == s.passJWTUsername {
		data["client_assertion_type"] = []string{"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"}
		data["client_assertion"] = []string{password}
		delete(data, "username")
		delete(data, "password")
	}

	s.l.WithFields(log.Fields{
		"service": service,
		"scope":   scope,
		"user":    user,
		"remote":  r.Header.Get("X-Forwarded-For"),
	}).Info("token request")

	req, err := http.NewRequest("POST", s.tokenUrl, strings.NewReader(data.Encode()))
	if err != nil {
		s.l.WithError(err).Warning("failed to create token request")
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-Length", strconv.Itoa(len(data.Encode())))
	req.Header.Set("User-Agent", r.UserAgent())
	req = traceRequest(req)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		s.l.WithError(err).Warning("failed to send token request")
		return
	}
	var tr TokenResponse
	err = json.NewDecoder(res.Body).Decode(&tr)
	if err != nil {
		s.l.WithError(err).Warning("failed to parse token response")
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
		s.l.WithError(err).Warning("failed to write response")
	}
}
