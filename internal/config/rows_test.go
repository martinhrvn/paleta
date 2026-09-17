package config

import (
	"reflect"
	"testing"
)

func TestLocation_DisplayName(t *testing.T) {
	if got := (Location{Name: "web", Location: "/repo/web"}).DisplayName(); got != "web" {
		t.Errorf("named: %q, want web", got)
	}
	if got := (Location{Location: "/repo/web"}).DisplayName(); got != "/repo/web" {
		t.Errorf("unnamed: %q, want the path", got)
	}
}

func rowsFixture() *Config {
	return &Config{
		Locations: []Location{
			{
				Name:     "web",
				Location: "/repo/web",
				Types:    Types{"npm", "docker"},
				Focused:  true,
				Env:      map[string]string{"PORT": "3000"},
				Commands: []Command{
					{Name: "build", Command: "npm run build", Type: "npm"},
					{Name: "build", Command: "docker build .", Type: "docker"},
					{Name: "deploy", Command: "./deploy.sh", Env: map[string]string{"STAGE": "prod"}},
				},
			},
			{
				Location:     "/repo/infra",
				PendingTypes: []string{"make"},
				Commands:     []Command{{Command: "terraform plan"}},
			},
		},
		ResolvedTools: []ResolvedTool{
			{Tool: "lazygit", Display: "lazygit", Command: "lazygit", Directory: "/w"},
			{Tool: "docker", Display: "docker: up", Command: "docker compose up", Directory: "/w"},
		},
	}
}

// Rows lists every location's commands in order, a placeholder for a location
// with types still resolving, and the tools last.
func TestConfig_Rows_Order(t *testing.T) {
	rows := rowsFixture().Rows(false)
	var labels []string
	for _, r := range rows {
		labels = append(labels, r.Label())
	}
	want := []string{
		"web: build", "web: build", "web: deploy",
		"/repo/infra: terraform plan", "/repo/infra: make",
		"lazygit", "docker: up",
	}
	if !reflect.DeepEqual(labels, want) {
		t.Errorf("labels = %q, want %q", labels, want)
	}

	pending := rows[4]
	if !reflect.DeepEqual(pending.Pending, []string{"make"}) || pending.Command != "" || pending.Directory != "/repo/infra" {
		t.Errorf("placeholder row = %+v", pending)
	}
	if !rows[5].IsTool || rows[5].Directory != "/w" || rows[6].Name != "up" || rows[6].DisplayName != "docker" {
		t.Errorf("tool rows = %+v, %+v", rows[5], rows[6])
	}
}

// With focusedOnly, unfocused locations drop out but tools stay; when nothing is
// focused the filter is a no-op rather than an empty list.
func TestConfig_Rows_Focus(t *testing.T) {
	cfg := rowsFixture()
	rows := cfg.Rows(true)
	for _, r := range rows {
		if !r.IsTool && r.DisplayName != "web" {
			t.Errorf("unfocused row leaked through: %+v", r)
		}
	}
	if len(rows) != 5 {
		t.Errorf("rows = %d, want web's 3 plus 2 tools", len(rows))
	}

	cfg.Locations[0].Focused = false
	if got := len(cfg.Rows(true)); got != 7 {
		t.Errorf("with nothing focused rows = %d, want all 7", got)
	}
}

// A row's command label carries the type only when the location declares
// several types; its plain label never does.
func TestRow_Labels(t *testing.T) {
	rows := rowsFixture().Rows(false)
	cases := []struct{ label, commandLabel string }{
		{"web: build", "[npm] build"},
		{"web: build", "[docker] build"},
		{"web: deploy", "deploy"},
		{"/repo/infra: terraform plan", "terraform plan"},
	}
	for i, tc := range cases {
		if rows[i].Label() != tc.label || rows[i].CommandLabel() != tc.commandLabel {
			t.Errorf("row %d: Label %q CommandLabel %q, want %q / %q", i, rows[i].Label(), rows[i].CommandLabel(), tc.label, tc.commandLabel)
		}
	}
	single := (&Config{Locations: []Location{{Name: "api", Types: Types{"go"}, Commands: []Command{{Name: "build", Type: "go"}}}}}).Rows(false)
	if got := single[0].CommandLabel(); got != "build" {
		t.Errorf("single-type command label = %q, want no prefix", got)
	}
	if got := rows[5].CommandLabel(); got != "lazygit" {
		t.Errorf("single-command tool label = %q, want the command", got)
	}
}

// Env is resolved per row: location env merged with the command's.
func TestConfig_Rows_Env(t *testing.T) {
	rows := rowsFixture().Rows(false)
	if rows[2].Env["PORT"] != "3000" || rows[2].Env["STAGE"] != "prod" {
		t.Errorf("deploy env = %v, want PORT and STAGE", rows[2].Env)
	}
}

// Invalid prefers the command's name problem, then its unresolved alias, then
// the location's name problem; Error carries only the alias failure.
func TestConfig_Rows_InvalidPrecedence(t *testing.T) {
	cfg := &Config{Locations: []Location{{
		Name:      "my proj",
		NameError: "contains a space",
		Commands: []Command{
			{Name: "test ui", NameError: "contains a space", Error: "unknown command"},
			{Name: "chain", Error: "unknown command"},
			{Name: "build"},
		},
	}}}
	rows := cfg.Rows(false)
	if rows[0].Invalid != "contains a space" || rows[0].Error != "unknown command" {
		t.Errorf("row 0 = %+v", rows[0])
	}
	if rows[1].Invalid != "unknown command" || rows[1].Error != "unknown command" {
		t.Errorf("row 1 = %+v", rows[1])
	}
	if rows[2].Invalid != "contains a space" || rows[2].Error != "" {
		t.Errorf("row 2 = %+v", rows[2])
	}
}
