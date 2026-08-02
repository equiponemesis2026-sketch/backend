package integration_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func request(method, url, body string) (*http.Response, error) {
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

func requestWithToken(method, url, token, body string) (*http.Response, error) {
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return http.DefaultClient.Do(req)
}

func readBody(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	return result
}

func TestHealthEndpoint(t *testing.T) {
	s := newTestServer()
	defer s.Close()

	resp, err := request(http.MethodGet, s.URL+"/health", "")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body := readBody(t, resp)
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}
	if body["mongodb"] != "up" {
		t.Errorf("expected mongodb up, got %v", body["mongodb"])
	}
}

func TestRegister_EmptyFields(t *testing.T) {
	s := newTestServer()
	defer s.Close()

	resp, err := request(http.MethodPost, s.URL+"/api/v1/auth/register", `{}`)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestFullContactFlow(t *testing.T) {
	s := newTestServer()
	defer s.Close()

	// 1. Register
	registerBody := `{"name":"Test User","email":"flow@test.com","password":"12345678","phone":"555-0100"}`
	resp, err := request(http.MethodPost, s.URL+"/api/v1/auth/register", registerBody)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 on register, got %d", resp.StatusCode)
	}

	// 2. Login
	loginBody := `{"email":"flow@test.com","password":"12345678"}`
	resp, err = request(http.MethodPost, s.URL+"/api/v1/auth/login", loginBody)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d", resp.StatusCode)
	}

	loginResp := readBody(t, resp)
	data, ok := loginResp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("login response missing data field")
	}
	token, ok := data["access_token"].(string)
	if !ok || token == "" {
		t.Fatal("login response missing access_token")
	}

	// 3. Create contact
	contactBody := `{"name":"Maria","phone":"555-0200","email":"maria@test.com","relationship":"familiar"}`
	resp, err = requestWithToken(http.MethodPost, s.URL+"/api/v1/contacts", token, contactBody)
	if err != nil {
		t.Fatalf("create contact failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 on create contact, got %d: %s", resp.StatusCode, readBody(t, resp)["message"])
	}

	createResp := readBody(t, resp)
	contactData, ok := createResp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("create contact response missing data")
	}
	contactID, ok := contactData["contact_id"].(string)
	if !ok || contactID == "" {
		t.Fatal("create contact response missing contact_id")
	}

	// 4. List contacts
	resp, err = requestWithToken(http.MethodGet, s.URL+"/api/v1/contacts", token, "")
	if err != nil {
		t.Fatalf("list contacts failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on list contacts, got %d", resp.StatusCode)
	}

	listResp := readBody(t, resp)
	contacts, ok := listResp["data"].([]interface{})
	if !ok || len(contacts) != 1 {
		t.Fatalf("expected 1 contact, got %d", len(contacts))
	}

	// 5. Update contact
	updateBody := `{"name":"Maria Updated","relationship":"amiga"}`
	resp, err = requestWithToken(http.MethodPut, s.URL+"/api/v1/contacts/"+contactID, token, updateBody)
	if err != nil {
		t.Fatalf("update contact failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on update, got %d: %s", resp.StatusCode, readBody(t, resp)["message"])
	}

	updateResp := readBody(t, resp)
	updatedData, ok := updateResp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("update response missing data")
	}
	if updatedData["name"] != "Maria Updated" {
		t.Errorf("expected name 'Maria Updated', got %v", updatedData["name"])
	}
	if updatedData["relationship"] != "amiga" {
		t.Errorf("expected relationship 'amiga', got %v", updatedData["relationship"])
	}

	// 6. Delete contact
	resp, err = requestWithToken(http.MethodDelete, s.URL+"/api/v1/contacts/"+contactID, token, "")
	if err != nil {
		t.Fatalf("delete contact failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d", resp.StatusCode)
	}

	// 7. Verify empty list
	resp, err = requestWithToken(http.MethodGet, s.URL+"/api/v1/contacts", token, "")
	if err != nil {
		t.Fatalf("list contacts after delete failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	listAfterResp := readBody(t, resp)
	contactsAfter, _ := listAfterResp["data"].([]interface{})
	if len(contactsAfter) != 0 {
		t.Errorf("expected empty list after delete, got %d items", len(contactsAfter))
	}
}

func TestContactsUnauthorized(t *testing.T) {
	s := newTestServer()
	defer s.Close()

	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/contacts", ""},
		{http.MethodPost, "/api/v1/contacts", `{"name":"Test","phone":"555-0000"}`},
		{http.MethodPut, "/api/v1/contacts/fake-id", `{"name":"Test"}`},
		{http.MethodDelete, "/api/v1/contacts/fake-id", ""},
	}

	for _, ep := range endpoints {
		resp, err := request(ep.method, s.URL+ep.path, ep.body)
		if err != nil {
			t.Fatalf("request to %s %s failed: %v", ep.method, ep.path, err)
		}
		func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusUnauthorized {			t.Errorf("expected 401 for %s %s, got %d", ep.method, ep.path, resp.StatusCode)
		}
	}
}

func TestUpdateContact_NotFound(t *testing.T) {
	s := newTestServer()
	defer s.Close()

	// Register and login to get a token
	registerBody := `{"name":"Test","email":"notfound@test.com","password":"12345678","phone":"555-0100"}`
	resp, err := request(http.MethodPost, s.URL+"/api/v1/auth/register", registerBody)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	func() { _ = resp.Body.Close() }()

	resp, err = request(http.MethodPost, s.URL+"/api/v1/auth/login", `{"email":"notfound@test.com","password":"12345678"}`)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	loginResp := readBody(t, resp)
	func() { _ = resp.Body.Close() }()

	data := loginResp["data"].(map[string]interface{})
	token := data["access_token"].(string)

	resp, err = requestWithToken(http.MethodPut, s.URL+"/api/v1/contacts/nonexistent-id", token, `{"name":"New"}`)
	if err != nil {
		t.Fatalf("update request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteContact_NotFound(t *testing.T) {
	s := newTestServer()
	defer s.Close()

	registerBody := `{"name":"Test","email":"delnotfound@test.com","password":"12345678","phone":"555-0100"}`
	resp, err := request(http.MethodPost, s.URL+"/api/v1/auth/register", registerBody)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	func() { _ = resp.Body.Close() }()

	resp, err = request(http.MethodPost, s.URL+"/api/v1/auth/login", `{"email":"delnotfound@test.com","password":"12345678"}`)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	loginResp := readBody(t, resp)
	func() { _ = resp.Body.Close() }()

	data := loginResp["data"].(map[string]interface{})
	token := data["access_token"].(string)

	resp, err = requestWithToken(http.MethodDelete, s.URL+"/api/v1/contacts/nonexistent-id", token, "")
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}
