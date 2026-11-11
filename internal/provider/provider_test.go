package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"muumuu": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	t.Helper()
	for _, k := range []string{envToken, envBaseURL, "MUUMUU_DOMAIN_TEST_DOMAIN_ID", "MUUMUU_DOMAIN_TEST_DOMAIN_FQDN"} {
		if os.Getenv(k) == "" {
			t.Fatalf("%s must be set for acceptance tests", k)
		}
	}
}

const fakeDomainID = "MU00000001"

type fakeRecord struct {
	ID       int64  `json:"id"`
	FQDN     string `json:"fqdn"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority,omitempty"`
}

type fakeAPI struct {
	mu          sync.Mutex
	nextID      int64
	records     map[int64]*fakeRecord
	autoRenew   bool
	setupType   string
	nameservers []string
	requests    []string

	nameserverPatches int
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{
		nextID:      100,
		records:     map[int64]*fakeRecord{},
		setupType:   "muumuu_dns",
		nameservers: []string{"ns1.muumuu-domain.com.", "ns2.muumuu-domain.com."},
	}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	t.Setenv(envToken, "muu_pat_test")
	t.Setenv(envBaseURL, srv.URL)
	return f
}

func (f *fakeAPI) deleteAllRecords() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records = map[int64]*fakeRecord{}
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if r.Header.Get("Authorization") != "Bearer muu_pat_test" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"code": "unauthorized", "message": "unauthorized"}})
		return
	}

	domainPath := "/me/domains/" + fakeDomainID
	switch p := r.URL.Path; {
	case p == "/me/domains" && r.Method == http.MethodGet:
		data := []any{}
		if q := r.URL.Query().Get("fqdn"); q == "" || q == "example.com" {
			data = append(data, f.domain())
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data, "meta": map[string]int{"total": len(data), "page": 1, "page-size": 100}})
	case p == domainPath && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"data": f.domain()})
	case p == domainPath && r.Method == http.MethodPatch:
		var in struct {
			AutoRenewEnabled *bool `json:"auto-renew-enabled"`
		}
		decode(r, &in)
		if in.AutoRenewEnabled != nil {
			f.autoRenew = *in.AutoRenewEnabled
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": f.domain()})
	case p == domainPath+"/nameservers" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"data": f.nameserverSettings()})
	case p == domainPath+"/nameservers" && r.Method == http.MethodPatch:
		var in struct {
			SetupType   string   `json:"setup-type"`
			Nameservers []string `json:"nameservers"`
		}
		decode(r, &in)
		f.nameserverPatches++
		f.setupType = in.SetupType
		if in.SetupType == "custom" {
			f.nameservers = nil
			for _, ns := range in.Nameservers {
				f.nameservers = append(f.nameservers, ns+".")
			}
		} else {
			f.nameservers = []string{"ns1.muumuu-domain.com.", "ns2.muumuu-domain.com."}
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": f.nameserverSettings()})
	case p == domainPath+"/dns-records" && r.Method == http.MethodGet:
		data := []*fakeRecord{}
		for _, rec := range f.records {
			if t := r.URL.Query().Get("type"); t != "" && t != rec.Type {
				continue
			}
			data = append(data, rec)
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data, "meta": map[string]int{"total": len(data), "page": 1, "page-size": 100}})
	case p == domainPath+"/dns-records" && r.Method == http.MethodPost:
		var in fakeRecord
		decode(r, &in)
		f.nextID++
		in.ID = f.nextID
		in.FQDN += "."
		if in.Type == "CNAME" && !strings.HasSuffix(in.Value, ".") {
			in.Value += "."
		}
		if in.TTL == 0 {
			in.TTL = 3600
		}
		f.records[in.ID] = &in
		f.requests = append(f.requests, "POST "+strconv.FormatInt(in.ID, 10))
		writeJSON(w, http.StatusCreated, map[string]any{"data": in})
	case strings.HasPrefix(p, domainPath+"/dns-records/"):
		id, _ := strconv.ParseInt(strings.TrimPrefix(p, domainPath+"/dns-records/"), 10, 64)
		rec, ok := f.records[id]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "not_found", "message": "not found"}})
			return
		}
		switch r.Method {
		case http.MethodPatch:
			var in struct {
				Value    *string `json:"value"`
				Priority *int    `json:"priority"`
			}
			decode(r, &in)
			if in.Value != nil {
				rec.Value = *in.Value
			}
			if in.Priority != nil {
				rec.Priority = in.Priority
			}
			f.requests = append(f.requests, fmt.Sprintf("PATCH %d", id))
			writeJSON(w, http.StatusOK, map[string]any{"data": rec})
		case http.MethodDelete:
			delete(f.records, id)
			f.requests = append(f.requests, fmt.Sprintf("DELETE %d", id))
			w.WriteHeader(http.StatusNoContent)
		}
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "not_found", "message": r.Method + " " + p}})
	}
}

func (f *fakeAPI) domain() map[string]any {
	return map[string]any{
		"id":                  fakeDomainID,
		"sld":                 "example",
		"tld":                 "com",
		"fqdn":                "example.com",
		"state":               "active",
		"setup-state":         "completed",
		"registrar":           "gmo",
		"whois-proxy-enabled": true,
		"auto-renew-enabled":  f.autoRenew,
		"contract-end-date":   "2027-01-01",
	}
}

func (f *fakeAPI) nameserverSettings() map[string]any {
	return map[string]any{"domain-id": fakeDomainID, "setup-type": f.setupType, "nameservers": f.nameservers}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decode(r *http.Request, v any) {
	_ = json.NewDecoder(r.Body).Decode(v)
}

func importStateIDWithDomain(name string) func(*terraform.State) (string, error) {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return "", fmt.Errorf("resource %s not found in state", name)
		}
		return rs.Primary.Attributes["domain_id"] + "/" + rs.Primary.ID, nil
	}
}

func tfjsonPath(attr string) tfjsonpath.Path {
	return tfjsonpath.New(attr)
}
