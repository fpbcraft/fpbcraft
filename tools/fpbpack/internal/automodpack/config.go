package automodpack

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type Settings struct {
	ModpackHost               bool     `json:"modpack_host"`
	GenerateModpackOnStart    bool     `json:"generate_modpack_on_start"`
	AutoExcludeServerSideMods bool     `json:"auto_exclude_server_side_mods"`
	RequireModpack            bool     `json:"require_modpack"`
	AcceptedLoaders           []string `json:"accepted_loaders"`
	AdvertiseVersionsToSync   bool     `json:"advertise_versions_to_sync"`
	SelfUpdater               bool     `json:"self_updater"`
	ConnectionMode            string   `json:"connection_mode"`
	BindAddress               string   `json:"bind_address"`
	BindPort                  int      `json:"bind_port"`
	AdvertisedEndpointHost    string   `json:"advertised_endpoint_host"`
	AdvertisedEndpointPort    int      `json:"advertised_endpoint_port"`
	BandwidthLimit            int      `json:"bandwidth_limit"`
	DisableInternalTLS        bool     `json:"disable_internal_tls"`
	AcceptProxyProtocol       bool     `json:"accept_proxy_protocol"`
	ValidateSecrets           bool     `json:"validate_secrets"`
	SecretLifetime            int64    `json:"secret_lifetime"`
	ExportHTTPDirectory       string   `json:"export_http_directory"`
	ExportHTTPIncludeAll      bool     `json:"export_http_include_all"`
	NagUnmoddedClients        bool     `json:"nag_unmodded_clients"`
	NagMessage                string   `json:"nag_message"`
	NagClickableMessage       string   `json:"nag_clickable_message"`
	NagClickableLink          string   `json:"nag_clickable_link"`
}

type Group struct {
	ID                  string   `json:"id"`
	DisplayName         string   `json:"display_name"`
	Description         string   `json:"description"`
	Required            bool     `json:"required"`
	DefaultSelected     bool     `json:"default_selected"`
	Requires            []string `json:"requires"`
	BreaksWith          []string `json:"breaks_with"`
	CompatiblePlatforms []string `json:"compatible_platforms"`
	FromServer          []string `json:"from_server"`
	Exclude             []string `json:"exclude"`
	Editable            []string `json:"editable"`
}

type Category struct {
	Name   string  `json:"name"`
	Groups []Group `json:"groups"`
}

type Config struct {
	Name       string     `json:"name"`
	Settings   Settings   `json:"settings"`
	Categories []Category `json:"categories"`
}

type Finding struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Group   string `json:"group,omitempty"`
}

type valueKind int

const (
	kindScalar valueKind = iota
	kindList
	kindObject
)

type value struct {
	kind   valueKind
	scalar string
	list   []string
	object *object
}

type entry struct {
	key   string
	value value
}

type object struct {
	entries []entry
}

type Document struct {
	root object
}

func DefaultConfig() Config {
	return Config{
		Settings: Settings{
			AcceptedLoaders:           []string{},
			ModpackHost:               true,
			GenerateModpackOnStart:    true,
			AutoExcludeServerSideMods: true,
			RequireModpack:            true,
			AdvertiseVersionsToSync:   true,
			ConnectionMode:            "HOLEPUNCH",
			BindPort:                  -1,
			AdvertisedEndpointPort:    -1,
			ValidateSecrets:           true,
			SecretLifetime:            336,
			NagUnmoddedClients:        true,
			NagMessage:                "Install the AutoModpack mod to get this server's modpack!",
			NagClickableMessage:       "Click here to get the AutoModpack!",
			NagClickableLink:          "https://modrinth.com/project/automodpack",
		},
		Categories: []Category{{
			Name: "General",
			Groups: []Group{{
				ID:              "main",
				Description:     "Core modpack files",
				Required:        true,
				DefaultSelected: true,
				FromServer:      []string{"mods/*.jar", "kubejs/**", "emotes/*"},
				Exclude:         []string{"**/.*", "**/.*/**", "**/*.{tmp,disabled,bak}", "kubejs/server_scripts/**"},
				Editable:        []string{"options.txt", "config/**"},
			}},
		}},
	}
}

func Parse(content []byte) (*Document, error) {
	tokens, err := lex(string(content))
	if err != nil {
		return nil, err
	}
	parser := tokenParser{tokens: tokens}
	root, err := parser.parseObject(false)
	if err != nil {
		return nil, err
	}
	if parser.pos != len(tokens) {
		return nil, fmt.Errorf("unexpected token %q", tokens[parser.pos].text)
	}
	return &Document{root: root}, nil
}

func (d *Document) Config() Config {
	cfg := DefaultConfig()
	root := &d.root
	cfg.Settings.ModpackHost = boolValue(root, "modpack-host", cfg.Settings.ModpackHost)
	cfg.Settings.GenerateModpackOnStart = boolValue(root, "generate-modpack-on-start", cfg.Settings.GenerateModpackOnStart)
	cfg.Settings.AutoExcludeServerSideMods = boolValue(root, "auto-exclude-server-side-mods", cfg.Settings.AutoExcludeServerSideMods)
	cfg.Settings.RequireModpack = boolValue(root, "require-modpack", cfg.Settings.RequireModpack)
	cfg.Settings.AcceptedLoaders = listValue(root, "accepted-loaders", cfg.Settings.AcceptedLoaders)
	cfg.Settings.AdvertiseVersionsToSync = boolValue(root, "advertise-versions-to-sync", cfg.Settings.AdvertiseVersionsToSync)
	cfg.Settings.SelfUpdater = boolValue(root, "self-updater", cfg.Settings.SelfUpdater)
	cfg.Settings.ConnectionMode = stringValue(root, "connection-mode", cfg.Settings.ConnectionMode)
	cfg.Settings.BindAddress = stringValue(root, "bind-address", cfg.Settings.BindAddress)
	cfg.Settings.BindPort = intValue(root, "bind-port", cfg.Settings.BindPort)
	cfg.Settings.AdvertisedEndpointHost = stringValue(root, "advertised-endpoint-host", cfg.Settings.AdvertisedEndpointHost)
	cfg.Settings.AdvertisedEndpointPort = intValue(root, "advertised-endpoint-port", cfg.Settings.AdvertisedEndpointPort)
	cfg.Settings.BandwidthLimit = intValue(root, "bandwidth-limit", cfg.Settings.BandwidthLimit)
	cfg.Settings.DisableInternalTLS = boolValue(root, "disable-internal-tls", cfg.Settings.DisableInternalTLS)
	cfg.Settings.AcceptProxyProtocol = boolValue(root, "accept-proxy-protocol", cfg.Settings.AcceptProxyProtocol)
	cfg.Settings.ValidateSecrets = boolValue(root, "validate-secrets", cfg.Settings.ValidateSecrets)
	cfg.Settings.SecretLifetime = int64Value(root, "secret-lifetime", cfg.Settings.SecretLifetime)
	cfg.Settings.ExportHTTPDirectory = stringValue(root, "export-http-directory", cfg.Settings.ExportHTTPDirectory)
	cfg.Settings.ExportHTTPIncludeAll = boolValue(root, "export-http-include-all", cfg.Settings.ExportHTTPIncludeAll)
	cfg.Settings.NagUnmoddedClients = boolValue(root, "nag-un-modded-clients", cfg.Settings.NagUnmoddedClients)
	cfg.Settings.NagMessage = stringValue(root, "nag-message", cfg.Settings.NagMessage)
	cfg.Settings.NagClickableMessage = stringValue(root, "nag-clickable-message", cfg.Settings.NagClickableMessage)
	cfg.Settings.NagClickableLink = stringValue(root, "nag-clickable-link", cfg.Settings.NagClickableLink)

	modpack, ok := objectValue(root, "modpack")
	if !ok {
		return cfg
	}
	cfg.Name = stringValue(modpack, "name", "")
	cfg.Categories = []Category{}
	for _, categoryEntry := range modpack.entries {
		if categoryEntry.key == "name" || categoryEntry.value.kind != kindObject || categoryEntry.value.object == nil {
			continue
		}
		category := Category{Name: categoryEntry.key}
		for _, groupEntry := range categoryEntry.value.object.entries {
			if groupEntry.value.kind != kindObject || groupEntry.value.object == nil {
				continue
			}
			groupObject := groupEntry.value.object
			category.Groups = append(category.Groups, Group{
				ID:                  groupEntry.key,
				DisplayName:         stringValue(groupObject, "display-name", ""),
				Description:         stringValue(groupObject, "description", ""),
				Required:            boolValue(groupObject, "required", false),
				DefaultSelected:     boolValue(groupObject, "default-selected", false),
				Requires:            listValue(groupObject, "requires", nil),
				BreaksWith:          listValue(groupObject, "breaks-with", nil),
				CompatiblePlatforms: listValue(groupObject, "compatible-platforms", nil),
				FromServer:          listValue(groupObject, "from-server", nil),
				Exclude:             listValue(groupObject, "exclude", nil),
				Editable:            listValue(groupObject, "editable", nil),
			})
		}
		cfg.Categories = append(cfg.Categories, category)
	}
	return cfg
}

func (d *Document) Apply(cfg Config) error {
	findings := Validate(cfg)
	for _, finding := range findings {
		if finding.Level == "error" {
			return fmt.Errorf("%s", finding.Message)
		}
	}

	root := &d.root
	setScalar(root, "modpack-host", strconv.FormatBool(cfg.Settings.ModpackHost))
	setScalar(root, "generate-modpack-on-start", strconv.FormatBool(cfg.Settings.GenerateModpackOnStart))
	setScalar(root, "auto-exclude-server-side-mods", strconv.FormatBool(cfg.Settings.AutoExcludeServerSideMods))
	setScalar(root, "require-modpack", strconv.FormatBool(cfg.Settings.RequireModpack))
	setList(root, "accepted-loaders", cfg.Settings.AcceptedLoaders)
	setScalar(root, "advertise-versions-to-sync", strconv.FormatBool(cfg.Settings.AdvertiseVersionsToSync))
	setScalar(root, "self-updater", strconv.FormatBool(cfg.Settings.SelfUpdater))
	setScalar(root, "connection-mode", cfg.Settings.ConnectionMode)
	setScalar(root, "bind-address", cfg.Settings.BindAddress)
	setScalar(root, "bind-port", strconv.Itoa(cfg.Settings.BindPort))
	setScalar(root, "advertised-endpoint-host", cfg.Settings.AdvertisedEndpointHost)
	setScalar(root, "advertised-endpoint-port", strconv.Itoa(cfg.Settings.AdvertisedEndpointPort))
	setScalar(root, "bandwidth-limit", strconv.Itoa(cfg.Settings.BandwidthLimit))
	setScalar(root, "disable-internal-tls", strconv.FormatBool(cfg.Settings.DisableInternalTLS))
	setScalar(root, "accept-proxy-protocol", strconv.FormatBool(cfg.Settings.AcceptProxyProtocol))
	setScalar(root, "validate-secrets", strconv.FormatBool(cfg.Settings.ValidateSecrets))
	setScalar(root, "secret-lifetime", strconv.FormatInt(cfg.Settings.SecretLifetime, 10))
	setScalar(root, "export-http-directory", cfg.Settings.ExportHTTPDirectory)
	setScalar(root, "export-http-include-all", strconv.FormatBool(cfg.Settings.ExportHTTPIncludeAll))
	setScalar(root, "nag-un-modded-clients", strconv.FormatBool(cfg.Settings.NagUnmoddedClients))
	setScalar(root, "nag-message", cfg.Settings.NagMessage)
	setScalar(root, "nag-clickable-message", cfg.Settings.NagClickableMessage)
	setScalar(root, "nag-clickable-link", cfg.Settings.NagClickableLink)

	oldModpack, _ := objectValue(root, "modpack")
	if oldModpack == nil {
		oldModpack = &object{}
	}
	newModpack := &object{}
	setScalar(newModpack, "name", cfg.Name)
	for _, category := range cfg.Categories {
		categoryObject := &object{}
		oldCategory, _ := objectValue(oldModpack, category.Name)
		for _, group := range category.Groups {
			var groupObject *object
			if oldCategory != nil {
				if existing, ok := objectValue(oldCategory, group.ID); ok {
					groupObject = cloneObject(existing)
				}
			}
			if groupObject == nil {
				groupObject = &object{}
			}
			setScalar(groupObject, "display-name", group.DisplayName)
			setScalar(groupObject, "description", group.Description)
			setScalar(groupObject, "required", strconv.FormatBool(group.Required))
			setScalar(groupObject, "default-selected", strconv.FormatBool(group.DefaultSelected))
			setList(groupObject, "requires", group.Requires)
			setList(groupObject, "breaks-with", group.BreaksWith)
			setList(groupObject, "compatible-platforms", group.CompatiblePlatforms)
			setList(groupObject, "from-server", group.FromServer)
			setList(groupObject, "exclude", group.Exclude)
			setList(groupObject, "editable", group.Editable)
			setObject(categoryObject, group.ID, groupObject)
		}
		setObject(newModpack, category.Name, categoryObject)
	}
	setObject(root, "modpack", newModpack)
	return nil
}

func (d *Document) Render() []byte {
	var builder strings.Builder
	renderObject(&builder, &d.root, 0, false)
	if builder.Len() == 0 || builder.String()[builder.Len()-1] != '\n' {
		builder.WriteByte('\n')
	}
	return []byte(builder.String())
}

func Validate(cfg Config) []Finding {
	findings := []Finding{}
	categoryNames := map[string]string{}
	groupIDs := map[string]struct{}{}
	groups := map[string]Group{}

	for _, category := range cfg.Categories {
		name := category.Name
		trimmed := strings.TrimSpace(name)
		switch {
		case trimmed == "":
			findings = append(findings, Finding{Level: "error", Code: "category_blank", Message: "AutoModpack category names cannot be blank."})
		case trimmed != name:
			findings = append(findings, Finding{Level: "error", Code: "category_whitespace", Message: fmt.Sprintf("Category %q has leading or trailing whitespace.", name)})
		case len([]rune(name)) > 64:
			findings = append(findings, Finding{Level: "error", Code: "category_too_long", Message: fmt.Sprintf("Category %q is longer than 64 characters.", name)})
		}
		for _, char := range name {
			if unicode.IsControl(char) {
				findings = append(findings, Finding{Level: "error", Code: "category_control", Message: fmt.Sprintf("Category %q contains a control character.", name)})
				break
			}
		}
		key := strings.ToLower(name)
		if previous, exists := categoryNames[key]; exists {
			findings = append(findings, Finding{Level: "error", Code: "category_duplicate", Message: fmt.Sprintf("Categories %q and %q are duplicates ignoring case.", previous, name)})
		} else {
			categoryNames[key] = name
		}

		for _, group := range category.Groups {
			if err := validateGroupID(group.ID); err != nil {
				findings = append(findings, Finding{Level: "error", Code: "group_id_invalid", Group: group.ID, Message: err.Error()})
				continue
			}
			if _, exists := groupIDs[group.ID]; exists {
				findings = append(findings, Finding{Level: "error", Code: "group_id_duplicate", Group: group.ID, Message: fmt.Sprintf("AutoModpack group id %q is declared more than once.", group.ID)})
			}
			groupIDs[group.ID] = struct{}{}
			groups[group.ID] = group
			if group.Required && !group.DefaultSelected {
				findings = append(findings, Finding{Level: "info", Code: "required_default_ignored", Group: group.ID, Message: fmt.Sprintf("Group %q is required, so default-selected has no effect.", group.ID)})
			}
			for _, platform := range group.CompatiblePlatforms {
				if platform == "" || platform != strings.ToLower(platform) || strings.IndexFunc(platform, unicode.IsSpace) >= 0 {
					findings = append(findings, Finding{Level: "error", Code: "platform_invalid", Group: group.ID, Message: fmt.Sprintf("Group %q has invalid platform %q; platform names must be short lowercase names.", group.ID, platform)})
				}
			}
		}
	}

	if _, ok := groupIDs["main"]; !ok {
		findings = append(findings, Finding{Level: "warning", Code: "main_missing", Message: "No group with id \"main\" is declared. AutoModpack's factory configuration uses main for core content."})
	}

	for id, group := range groups {
		requires := map[string]struct{}{}
		for _, required := range group.Requires {
			if _, exists := groupIDs[required]; !exists {
				findings = append(findings, Finding{Level: "error", Code: "requires_missing", Group: id, Message: fmt.Sprintf("Group %q requires unknown group %q.", id, required)})
			}
			if required == id {
				findings = append(findings, Finding{Level: "error", Code: "requires_self", Group: id, Message: fmt.Sprintf("Group %q cannot require itself.", id)})
			}
			requires[required] = struct{}{}
		}
		for _, broken := range group.BreaksWith {
			if _, exists := groupIDs[broken]; !exists {
				findings = append(findings, Finding{Level: "error", Code: "breaks_missing", Group: id, Message: fmt.Sprintf("Group %q conflicts with unknown group %q.", id, broken)})
			}
			if broken == id {
				findings = append(findings, Finding{Level: "error", Code: "breaks_self", Group: id, Message: fmt.Sprintf("Group %q cannot break with itself.", id)})
			}
			if _, exists := requires[broken]; exists {
				findings = append(findings, Finding{Level: "error", Code: "requires_breaks_conflict", Group: id, Message: fmt.Sprintf("Group %q both requires and breaks with %q.", id, broken)})
			}
		}
	}

	state := map[string]int{}
	var visit func(string, []string)
	visit = func(id string, stack []string) {
		if state[id] == 2 {
			return
		}
		if state[id] == 1 {
			cycle := append(stack, id)
			findings = append(findings, Finding{Level: "error", Code: "requires_cycle", Group: id, Message: "AutoModpack group requirement cycle: " + strings.Join(cycle, " → ")})
			return
		}
		state[id] = 1
		group, exists := groups[id]
		if exists {
			for _, next := range group.Requires {
				if _, ok := groups[next]; ok {
					visit(next, append(stack, id))
				}
			}
		}
		state[id] = 2
	}
	ids := make([]string, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		visit(id, nil)
	}
	return findings
}

func validateGroupID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("AutoModpack group id cannot be blank")
	}
	if id != strings.TrimSpace(id) || id == "." || id == ".." || strings.ContainsAny(id, "/\\:") {
		return fmt.Errorf("invalid AutoModpack group id %q", id)
	}
	for _, char := range id {
		if unicode.IsSpace(char) || unicode.IsControl(char) {
			return fmt.Errorf("invalid AutoModpack group id %q", id)
		}
	}
	return nil
}

func objectValue(o *object, key string) (*object, bool) {
	if o == nil {
		return nil, false
	}
	for i := range o.entries {
		if o.entries[i].key == key && o.entries[i].value.kind == kindObject {
			return o.entries[i].value.object, o.entries[i].value.object != nil
		}
	}
	return nil, false
}

func scalarValue(o *object, key string) (string, bool) {
	if o == nil {
		return "", false
	}
	for _, item := range o.entries {
		if item.key == key && item.value.kind == kindScalar {
			return item.value.scalar, true
		}
	}
	return "", false
}

func stringValue(o *object, key, fallback string) string {
	if value, ok := scalarValue(o, key); ok {
		return value
	}
	return fallback
}

func boolValue(o *object, key string, fallback bool) bool {
	raw, ok := scalarValue(o, key)
	if !ok {
		return fallback
	}
	value, err := strconv.ParseBool(strings.ToLower(raw))
	if err != nil {
		return fallback
	}
	return value
}

func intValue(o *object, key string, fallback int) int {
	raw, ok := scalarValue(o, key)
	if !ok {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func int64Value(o *object, key string, fallback int64) int64 {
	raw, ok := scalarValue(o, key)
	if !ok {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func listValue(o *object, key string, fallback []string) []string {
	if o == nil {
		return nonNilStrings(fallback)
	}
	for _, item := range o.entries {
		if item.key == key && item.value.kind == kindList {
			return nonNilStrings(item.value.list)
		}
	}
	return nonNilStrings(fallback)
}

func nonNilStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string{}, values...)
}

func setScalar(o *object, key, scalar string) {
	setValue(o, key, value{kind: kindScalar, scalar: scalar})
}

func setList(o *object, key string, list []string) {
	setValue(o, key, value{kind: kindList, list: append([]string(nil), list...)})
}

func setObject(o *object, key string, child *object) {
	setValue(o, key, value{kind: kindObject, object: child})
}

func setValue(o *object, key string, next value) {
	for index := range o.entries {
		if o.entries[index].key == key {
			o.entries[index].value = next
			return
		}
	}
	o.entries = append(o.entries, entry{key: key, value: next})
}

func cloneObject(source *object) *object {
	if source == nil {
		return &object{}
	}
	result := &object{entries: make([]entry, 0, len(source.entries))}
	for _, item := range source.entries {
		copied := item
		copied.value.list = append([]string(nil), item.value.list...)
		if item.value.object != nil {
			copied.value.object = cloneObject(item.value.object)
		}
		result.entries = append(result.entries, copied)
	}
	return result
}

func renderObject(builder *strings.Builder, o *object, indent int, braces bool) {
	if braces {
		builder.WriteString("{\n")
	}
	for _, item := range o.entries {
		for i := 0; i < indent; i++ {
			builder.WriteString("  ")
		}
		builder.WriteString(renderKey(item.key))
		switch item.value.kind {
		case kindObject:
			builder.WriteString(" ")
			renderObject(builder, item.value.object, indent+1, true)
			for i := 0; i < indent; i++ {
				builder.WriteString("  ")
			}
			builder.WriteString("}\n")
		case kindList:
			builder.WriteString(": [")
			for index, listItem := range item.value.list {
				if index > 0 {
					builder.WriteString(", ")
				}
				builder.WriteString(strconv.Quote(listItem))
			}
			builder.WriteString("]\n")
		default:
			builder.WriteString(": ")
			if isLiteralScalar(item.value.scalar) {
				builder.WriteString(item.value.scalar)
			} else {
				builder.WriteString(strconv.Quote(item.value.scalar))
			}
			builder.WriteByte('\n')
		}
	}
}

func renderKey(key string) string {
	if key != "" {
		valid := true
		for _, char := range key {
			if !(unicode.IsLetter(char) || unicode.IsDigit(char) || char == '-' || char == '_' || char == '.') {
				valid = false
				break
			}
		}
		if valid {
			return key
		}
	}
	return strconv.Quote(key)
}

func isLiteralScalar(value string) bool {
	if value == "true" || value == "false" || value == "null" {
		return true
	}
	if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return true
	}
	return false
}

type tokenKind int

const (
	tokenWord tokenKind = iota
	tokenLBrace
	tokenRBrace
	tokenLBracket
	tokenRBracket
	tokenColon
	tokenComma
	tokenNewline
)

type token struct {
	kind   tokenKind
	text   string
	quoted bool
}

func lex(input string) ([]token, error) {
	tokens := []token{}
	for index := 0; index < len(input); {
		char := input[index]
		if char == '\r' || char == '\n' {
			if char == '\r' && index+1 < len(input) && input[index+1] == '\n' {
				index++
			}
			tokens = append(tokens, token{kind: tokenNewline, text: "\n"})
			index++
			continue
		}
		if unicode.IsSpace(rune(char)) {
			index++
			continue
		}
		if char == '#' {
			for index < len(input) && input[index] != '\r' && input[index] != '\n' {
				index++
			}
			continue
		}
		switch char {
		case '{':
			tokens = append(tokens, token{kind: tokenLBrace, text: "{"})
			index++
		case '}':
			tokens = append(tokens, token{kind: tokenRBrace, text: "}"})
			index++
		case '[':
			tokens = append(tokens, token{kind: tokenLBracket, text: "["})
			index++
		case ']':
			tokens = append(tokens, token{kind: tokenRBracket, text: "]"})
			index++
		case ':', '=':
			tokens = append(tokens, token{kind: tokenColon, text: string(char)})
			index++
		case ',':
			tokens = append(tokens, token{kind: tokenComma, text: ","})
			index++
		case '"':
			start := index
			index++
			escaped := false
			for index < len(input) {
				current := input[index]
				if escaped {
					escaped = false
					index++
					continue
				}
				if current == '\\' {
					escaped = true
					index++
					continue
				}
				if current == '"' {
					index++
					break
				}
				index++
			}
			if index > len(input) || input[index-1] != '"' {
				return nil, fmt.Errorf("unterminated quoted string near byte %d", start)
			}
			value, err := strconv.Unquote(input[start:index])
			if err != nil {
				return nil, fmt.Errorf("invalid quoted string near byte %d: %w", start, err)
			}
			tokens = append(tokens, token{kind: tokenWord, text: value, quoted: true})
		default:
			start := index
			for index < len(input) {
				current := input[index]
				if current == '\r' || current == '\n' || unicode.IsSpace(rune(current)) || strings.ContainsRune("{}[]:=,#", rune(current)) {
					break
				}
				index++
			}
			if start == index {
				return nil, fmt.Errorf("unexpected character %q at byte %d", char, index)
			}
			tokens = append(tokens, token{kind: tokenWord, text: input[start:index]})
		}
	}
	return tokens, nil
}

type tokenParser struct {
	tokens []token
	pos    int
}

func (p *tokenParser) skipEntrySeparators() {
	for p.pos < len(p.tokens) && (p.tokens[p.pos].kind == tokenNewline || p.tokens[p.pos].kind == tokenComma) {
		p.pos++
	}
}

func (p *tokenParser) skipNewlines() {
	for p.pos < len(p.tokens) && p.tokens[p.pos].kind == tokenNewline {
		p.pos++
	}
}

func (p *tokenParser) parseObject(expectClosing bool) (object, error) {
	result := object{}
	for p.pos < len(p.tokens) {
		p.skipEntrySeparators()
		if p.pos >= len(p.tokens) {
			break
		}
		if p.tokens[p.pos].kind == tokenRBrace {
			if !expectClosing {
				return object{}, fmt.Errorf("unexpected closing brace")
			}
			p.pos++
			return result, nil
		}
		if p.tokens[p.pos].kind != tokenWord {
			return object{}, fmt.Errorf("expected configuration key, got %q", p.tokens[p.pos].text)
		}
		key := p.tokens[p.pos].text
		p.pos++
		if p.pos >= len(p.tokens) {
			return object{}, fmt.Errorf("missing value for %q", key)
		}
		if p.tokens[p.pos].kind == tokenColon {
			p.pos++
		}
		p.skipNewlines()
		if p.pos >= len(p.tokens) {
			return object{}, fmt.Errorf("missing value for %q", key)
		}
		next, err := p.parseValue()
		if err != nil {
			return object{}, fmt.Errorf("%s: %w", key, err)
		}
		result.entries = append(result.entries, entry{key: key, value: next})
	}
	if expectClosing {
		return object{}, fmt.Errorf("unterminated object")
	}
	return result, nil
}

func (p *tokenParser) parseValue() (value, error) {
	if p.pos >= len(p.tokens) {
		return value{}, fmt.Errorf("missing value")
	}
	current := p.tokens[p.pos]
	switch current.kind {
	case tokenLBrace:
		p.pos++
		child, err := p.parseObject(true)
		if err != nil {
			return value{}, err
		}
		return value{kind: kindObject, object: &child}, nil
	case tokenLBracket:
		return p.parseList()
	case tokenWord:
		scalar, err := p.parseScalar(tokenNewline, tokenRBrace, tokenComma)
		if err != nil {
			return value{}, err
		}
		return value{kind: kindScalar, scalar: scalar}, nil
	default:
		return value{}, fmt.Errorf("unexpected value token %q", current.text)
	}
}

func (p *tokenParser) parseList() (value, error) {
	p.pos++ // [
	items := []string{}
	for {
		p.skipEntrySeparators()
		if p.pos >= len(p.tokens) {
			return value{}, fmt.Errorf("unterminated list")
		}
		if p.tokens[p.pos].kind == tokenRBracket {
			p.pos++
			return value{kind: kindList, list: items}, nil
		}
		if p.tokens[p.pos].kind != tokenWord {
			return value{}, fmt.Errorf("lists may contain only scalar values")
		}
		item, err := p.parseScalar(tokenComma, tokenRBracket, tokenNewline)
		if err != nil {
			return value{}, err
		}
		items = append(items, item)
		if p.pos < len(p.tokens) && p.tokens[p.pos].kind == tokenComma {
			p.pos++
		}
	}
}

func (p *tokenParser) parseScalar(stoppers ...tokenKind) (string, error) {
	stop := map[tokenKind]bool{}
	for _, kind := range stoppers {
		stop[kind] = true
	}
	if p.pos >= len(p.tokens) || p.tokens[p.pos].kind != tokenWord {
		return "", fmt.Errorf("expected scalar value")
	}

	var builder strings.Builder
	hadValue := false
	previous := tokenColon
	for p.pos < len(p.tokens) {
		current := p.tokens[p.pos]
		if stop[current.kind] {
			break
		}
		switch current.kind {
		case tokenWord:
			// Preserve compatibility with the compact colon-less object syntax
			// accepted by AutoModpack/Reconf (for example: name: "Pack" General { ... }).
			// A word followed by ':' or '{' after an already-started scalar is
			// therefore the next member key, not part of the current scalar.
			if hadValue && previous == tokenWord && p.pos+1 < len(p.tokens) &&
				(p.tokens[p.pos+1].kind == tokenColon || p.tokens[p.pos+1].kind == tokenLBrace) {
				return builder.String(), nil
			}
			if hadValue && previous == tokenWord {
				builder.WriteByte(' ')
			}
			builder.WriteString(current.text)
			hadValue = true
		case tokenColon:
			builder.WriteString(current.text)
		default:
			return "", fmt.Errorf("unexpected token %q in scalar value", current.text)
		}
		previous = current.kind
		p.pos++
	}
	if !hadValue {
		return "", fmt.Errorf("empty scalar value")
	}
	return builder.String(), nil
}
