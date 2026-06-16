package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"

	"golang.org/x/crypto/ssh"
)

const (
	nodeProvisionSSHTimeout     = 15 * time.Second
	nodeProvisionInstallTimeout = 10 * time.Minute
	nodeProvisionOutputLimit    = 80
)

type NodeProvisionRequest struct {
	Name                string `json:"name" validate:"required"`
	Remark              string `json:"remark"`
	SSHHost             string `json:"sshHost" validate:"required"`
	SSHPort             int    `json:"sshPort" validate:"gte=1,lte=65535"`
	SSHUser             string `json:"sshUser" validate:"required"`
	SSHPassword         string `json:"sshPassword"`
	SSHPrivateKey       string `json:"sshPrivateKey"`
	SSHPrivateKeyPass   string `json:"sshPrivateKeyPass"`
	SSHHostKeySHA256    string `json:"sshHostKeySha256"`
	SSHSkipHostKeyCheck bool   `json:"sshSkipHostKeyCheck"`
	SudoPassword        string `json:"sudoPassword"`
	PanelPort           int    `json:"panelPort" validate:"omitempty,gte=1,lte=65535"`
	WebBasePath         string `json:"webBasePath"`
	SSLMode             string `json:"sslMode" validate:"omitempty,oneof=none ip domain"`
	Domain              string `json:"domain"`
	ACMEEmail           string `json:"acmeEmail"`
	AllowPrivateAddress bool   `json:"allowPrivateAddress"`
	TlsVerifyMode       string `json:"tlsVerifyMode" validate:"omitempty,oneof=verify skip pin"`
	PinnedCertSha256    string `json:"pinnedCertSha256"`
}

type NodeProvisionResult struct {
	Node      *model.Node `json:"node,omitempty"`
	AccessURL string      `json:"accessUrl,omitempty"`
	Output    []string    `json:"output,omitempty"`
}

type installResult struct {
	Username    string
	PanelPort   int
	WebBasePath string
	AccessURL   string
	APIToken    string
}

type NodeProvisionService struct {
	NodeService NodeService
}

func (s *NodeProvisionService) Provision(ctx context.Context, req *NodeProvisionRequest) (*NodeProvisionResult, error) {
	if req == nil {
		return nil, common.NewError("request is required")
	}
	if err := req.normalize(); err != nil {
		return nil, err
	}

	raw, err := runProvisionSSH(ctx, req)
	redacted := redactProvisionOutput(raw)
	if err != nil {
		return &NodeProvisionResult{Output: redacted}, err
	}
	parsed, err := parseInstallResult(raw)
	if err != nil {
		return &NodeProvisionResult{Output: redacted}, err
	}

	n, err := req.toNode(parsed)
	if err != nil {
		return &NodeProvisionResult{AccessURL: parsed.AccessURL, Output: redacted}, err
	}
	if err := s.NodeService.normalize(n); err != nil {
		return &NodeProvisionResult{AccessURL: parsed.AccessURL, Output: redacted}, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	if _, err := s.NodeService.Probe(probeCtx, n); err != nil {
		return &NodeProvisionResult{AccessURL: parsed.AccessURL, Output: redacted}, errors.New(FriendlyProbeError(err.Error()))
	}
	if err := s.NodeService.Create(n); err != nil {
		return &NodeProvisionResult{AccessURL: parsed.AccessURL, Output: redacted}, err
	}
	return &NodeProvisionResult{Node: n, AccessURL: parsed.AccessURL, Output: redacted}, nil
}

func (r *NodeProvisionRequest) normalize() error {
	r.Name = strings.TrimSpace(r.Name)
	r.Remark = strings.TrimSpace(r.Remark)
	r.SSHHost = strings.TrimSpace(r.SSHHost)
	r.SSHUser = strings.TrimSpace(r.SSHUser)
	r.SSHPassword = strings.TrimSpace(r.SSHPassword)
	r.SSHPrivateKey = strings.TrimSpace(r.SSHPrivateKey)
	r.SSHPrivateKeyPass = strings.TrimSpace(r.SSHPrivateKeyPass)
	r.SSHHostKeySHA256 = normalizeSSHFingerprint(r.SSHHostKeySHA256)
	r.SudoPassword = strings.TrimSpace(r.SudoPassword)
	r.WebBasePath = strings.TrimSpace(r.WebBasePath)
	r.SSLMode = strings.TrimSpace(r.SSLMode)
	r.Domain = strings.TrimSpace(r.Domain)
	r.ACMEEmail = strings.TrimSpace(r.ACMEEmail)
	r.TlsVerifyMode = strings.TrimSpace(r.TlsVerifyMode)
	r.PinnedCertSha256 = strings.TrimSpace(r.PinnedCertSha256)
	if r.SSHPort == 0 {
		r.SSHPort = 22
	}
	if r.SSLMode == "" {
		r.SSLMode = "none"
	}
	if r.TlsVerifyMode == "" {
		r.TlsVerifyMode = "verify"
	}
	if r.SSLMode == "domain" && r.Domain == "" {
		return common.NewError("domain is required for domain SSL mode")
	}
	if r.SSHPassword == "" && r.SSHPrivateKey == "" {
		return common.NewError("ssh password or private key is required")
	}
	if r.SSHSkipHostKeyCheck {
		r.SSHHostKeySHA256 = ""
	} else if r.SSHHostKeySHA256 == "" {
		return common.NewError("ssh host key SHA256 fingerprint is required")
	} else if _, err := base64.StdEncoding.DecodeString(r.SSHHostKeySHA256); err != nil {
		return common.NewError("ssh host key SHA256 fingerprint must be base64")
	}
	host, err := netsafe.NormalizeHost(r.SSHHost)
	if err != nil {
		return common.NewError(err.Error())
	}
	r.SSHHost = host
	return nil
}

func (r *NodeProvisionRequest) toNode(inst installResult) (*model.Node, error) {
	host := r.SSHHost
	port := inst.PanelPort
	basePath := inst.WebBasePath
	scheme := "http"
	if inst.AccessURL != "" {
		u, err := url.Parse(inst.AccessURL)
		if err == nil {
			if u.Scheme == "http" || u.Scheme == "https" {
				scheme = u.Scheme
			}
			if u.Hostname() != "" && u.Hostname() != "SERVER_IP_UNKNOWN" {
				host = u.Hostname()
			}
			if u.Port() != "" {
				if p, convErr := strconv.Atoi(u.Port()); convErr == nil {
					port = p
				}
			}
			if u.Path != "" {
				basePath = strings.Trim(u.Path, "/")
			}
		}
	}
	if port == 0 {
		return nil, common.NewError("installer did not return panel port")
	}
	if inst.APIToken == "" {
		return nil, common.NewError("installer did not return API token")
	}
	return &model.Node{
		Name:                r.Name,
		Remark:              r.Remark,
		Scheme:              scheme,
		Address:             host,
		Port:                port,
		BasePath:            basePath,
		ApiToken:            inst.APIToken,
		Enable:              true,
		AllowPrivateAddress: r.AllowPrivateAddress,
		TlsVerifyMode:       r.TlsVerifyMode,
		PinnedCertSha256:    r.PinnedCertSha256,
		InboundSyncMode:     "all",
	}, nil
}

func runProvisionSSH(ctx context.Context, req *NodeProvisionRequest) (string, error) {
	auth, err := sshAuthMethods(req)
	if err != nil {
		return "", err
	}
	hostKeyCallback := pinnedSSHHostKeyCallback(req.SSHHostKeySHA256)
	hostKeyAlgorithms := []string{ssh.KeyAlgoED25519}
	if req.SSHSkipHostKeyCheck {
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
		hostKeyAlgorithms = nil
	}
	cfg := &ssh.ClientConfig{
		User:              req.SSHUser,
		Auth:              auth,
		HostKeyAlgorithms: hostKeyAlgorithms,
		HostKeyCallback:   hostKeyCallback,
		Timeout:           nodeProvisionSSHTimeout,
	}
	addr := net.JoinHostPort(req.SSHHost, strconv.Itoa(req.SSHPort))
	dialCtx, cancel := context.WithTimeout(ctx, nodeProvisionSSHTimeout)
	defer cancel()
	type dialResult struct {
		client *ssh.Client
		err    error
	}
	ch := make(chan dialResult, 1)
	go func() {
		conn, err := netsafe.SSRFGuardedDialContext(netsafe.ContextWithAllowPrivate(dialCtx, req.AllowPrivateAddress), "tcp", addr)
		if err != nil {
			ch <- dialResult{err: err}
			return
		}
		_ = conn.SetDeadline(time.Now().Add(nodeProvisionSSHTimeout))
		cconn, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
		if err != nil {
			_ = conn.Close()
			ch <- dialResult{err: err}
			return
		}
		_ = conn.SetDeadline(time.Time{})
		ch <- dialResult{client: ssh.NewClient(cconn, chans, reqs)}
	}()
	var client *ssh.Client
	select {
	case <-dialCtx.Done():
		return "", dialCtx.Err()
	case res := <-ch:
		if res.err != nil {
			return "", res.err
		}
		client = res.client
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var out bytes.Buffer
	session.Stdout = &out
	session.Stderr = &out
	command := buildProvisionCommand(req)
	runCtx, stop := context.WithTimeout(ctx, nodeProvisionInstallTimeout)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- session.Run(command) }()
	select {
	case <-runCtx.Done():
		_ = session.Signal(ssh.SIGKILL)
		return out.String(), runCtx.Err()
	case err := <-done:
		if err != nil {
			return out.String(), err
		}
	}
	if !strings.Contains(out.String(), "XUI_API_TOKEN=") {
		if extra := readProvisionInstallResult(client, req); extra != "" {
			out.WriteString(extra)
		}
	}
	return out.String(), nil
}

func readProvisionInstallResult(client *ssh.Client, req *NodeProvisionRequest) string {
	session, err := client.NewSession()
	if err != nil {
		return ""
	}
	defer session.Close()

	var out bytes.Buffer
	session.Stdout = &out
	session.Stderr = &out
	if err := session.Run(buildProvisionResultReadCommand(req)); err != nil {
		return ""
	}
	return out.String()
}

func buildProvisionResultReadCommand(req *NodeProvisionRequest) string {
	if req.SudoPassword != "" {
		return fmt.Sprintf("printf %%s\\\\n %s | sudo -S cat /etc/x-ui/install-result.env\n", shellQuote(req.SudoPassword))
	}
	if req.SSHUser == "root" {
		return "cat /etc/x-ui/install-result.env\n"
	}
	return "sudo cat /etc/x-ui/install-result.env\n"
}

func pinnedSSHHostKeyCallback(expected string) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		sum := sha256.Sum256(key.Marshal())
		got := base64.StdEncoding.EncodeToString(sum[:])
		if got != expected {
			return fmt.Errorf("ssh host key fingerprint mismatch: got SHA256:%s", got)
		}
		return nil
	}
}

func sshAuthMethods(req *NodeProvisionRequest) ([]ssh.AuthMethod, error) {
	methods := make([]ssh.AuthMethod, 0, 3)
	if req.SSHPassword != "" {
		methods = append(methods, ssh.Password(req.SSHPassword), ssh.KeyboardInteractive(
			func(_ string, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = req.SSHPassword
				}
				return answers, nil
			},
		))
	}
	if req.SSHPrivateKey != "" {
		var signer ssh.Signer
		var err error
		key := []byte(req.SSHPrivateKey)
		if req.SSHPrivateKeyPass != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(req.SSHPrivateKeyPass))
		} else {
			signer, err = ssh.ParsePrivateKey(key)
		}
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if len(methods) == 0 {
		return nil, common.NewError("ssh password or private key is required")
	}
	return methods, nil
}

func buildProvisionCommand(req *NodeProvisionRequest) string {
	var b strings.Builder
	b.WriteString("set -euo pipefail\n")
	b.WriteString("export DEBIAN_FRONTEND=noninteractive\n")
	b.WriteString("export XUI_NONINTERACTIVE=1\n")
	envArgs := provisionEnvArgs(req)
	writeExport := func(name string, value string) {
		if value == "" {
			return
		}
		fmt.Fprintf(&b, "export %s=%s\n", name, shellQuote(value))
	}
	writeExport("XUI_SSL_MODE", req.SSLMode)
	writeExport("XUI_DOMAIN", req.Domain)
	writeExport("XUI_ACME_EMAIL", req.ACMEEmail)
	sourceValues := provisionSourceEnvValues()
	for _, key := range provisionSourceEnvKeys() {
		writeExport(key, sourceValues[key])
	}
	if req.PanelPort > 0 {
		writeExport("XUI_PANEL_PORT", strconv.Itoa(req.PanelPort))
	}
	writeExport("XUI_WEB_BASE_PATH", req.WebBasePath)
	installer := provisionCurlCommand(provisionRawURL("install.sh")) + " | bash"
	if req.SudoPassword != "" {
		fmt.Fprintf(&b, "printf %%s\\\\n %s | sudo -S env%s bash -lc %s\n", shellQuote(req.SudoPassword), envArgs, shellQuote(installer))
		b.WriteString("sudo cat /etc/x-ui/install-result.env\n")
	} else if req.SSHUser == "root" {
		fmt.Fprintf(&b, "bash -lc %s\n", shellQuote(installer))
		b.WriteString("cat /etc/x-ui/install-result.env\n")
	} else {
		fmt.Fprintf(&b, "sudo env%s bash -lc %s\n", envArgs, shellQuote(installer))
		b.WriteString("sudo cat /etc/x-ui/install-result.env\n")
	}
	return b.String()
}

func provisionEnvArgs(req *NodeProvisionRequest) string {
	values := map[string]string{
		"DEBIAN_FRONTEND":    "noninteractive",
		"XUI_NONINTERACTIVE": "1",
		"XUI_SSL_MODE":       req.SSLMode,
		"XUI_DOMAIN":         req.Domain,
		"XUI_ACME_EMAIL":     req.ACMEEmail,
		"XUI_WEB_BASE_PATH":  req.WebBasePath,
	}
	for key, value := range provisionSourceEnvValues() {
		values[key] = value
	}
	if req.PanelPort > 0 {
		values["XUI_PANEL_PORT"] = strconv.Itoa(req.PanelPort)
	}
	order := []string{
		"DEBIAN_FRONTEND",
		"XUI_NONINTERACTIVE",
		"XUI_SSL_MODE",
		"XUI_DOMAIN",
		"XUI_ACME_EMAIL",
		"XUI_PANEL_PORT",
		"XUI_WEB_BASE_PATH",
		"XUI_RELEASE_API_URL",
		"XUI_RELEASE_ASSET_URL_TEMPLATE",
		"XUI_RAW_BASE_URL",
		"XUI_DOWNLOAD_AUTH_HEADER",
	}
	var b strings.Builder
	for _, key := range order {
		value := values[key]
		if value == "" {
			continue
		}
		fmt.Fprintf(&b, " %s=%s", key, shellQuote(value))
	}
	return b.String()
}

func provisionSourceEnvValues() map[string]string {
	keys := provisionSourceEnvKeys()
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			values[key] = value
		}
	}
	return values
}

func provisionSourceEnvKeys() []string {
	return []string{
		"XUI_RELEASE_API_URL",
		"XUI_RELEASE_ASSET_URL_TEMPLATE",
		"XUI_RAW_BASE_URL",
		"XUI_DOWNLOAD_AUTH_HEADER",
	}
}

func provisionRawURL(path string) string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("XUI_RAW_BASE_URL")), "/")
	if base == "" {
		base = "https://raw.githubusercontent.com/MHSanaei/3x-ui/main"
	}
	return base + "/" + strings.TrimLeft(path, "/")
}

func provisionCurlCommand(url string) string {
	var b strings.Builder
	b.WriteString("curl")
	if header := strings.TrimSpace(os.Getenv("XUI_DOWNLOAD_AUTH_HEADER")); header != "" {
		b.WriteString(" -H ")
		b.WriteString(shellQuote(header))
	}
	b.WriteString(" -fsSL ")
	b.WriteString(shellQuote(url))
	return b.String()
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func parseInstallResult(raw string) (installResult, error) {
	var out installResult
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		k, v, ok := strings.Cut(line, "=")
		if !ok || !strings.HasPrefix(k, "XUI_") {
			continue
		}
		value := parseShellValue(v)
		switch k {
		case "XUI_USERNAME":
			out.Username = value
		case "XUI_PANEL_PORT":
			if p, err := strconv.Atoi(value); err == nil {
				out.PanelPort = p
			}
		case "XUI_WEB_BASE_PATH":
			out.WebBasePath = value
		case "XUI_ACCESS_URL":
			out.AccessURL = value
		case "XUI_API_TOKEN":
			out.APIToken = value
		}
	}
	if err := scanner.Err(); err != nil {
		return out, err
	}
	if out.APIToken == "" {
		return out, common.NewError("install result did not include XUI_API_TOKEN")
	}
	return out, nil
}

func parseShellValue(v string) string {
	v = strings.TrimSpace(v)
	if unquoted, err := strconv.Unquote(v); err == nil {
		return unquoted
	}
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return strings.ReplaceAll(v[1:len(v)-1], `'\''`, `'`)
	}
	return strings.Trim(v, "\"'")
}

func normalizeSSHFingerprint(s string) string {
	s = strings.TrimSpace(s)
	if decoded, ok := decodeBase64Text(s); ok {
		s = decoded
	}
	for _, field := range strings.Fields(s) {
		if strings.HasPrefix(field, "SHA256:") {
			s = field
			break
		}
	}
	s = strings.TrimPrefix(strings.TrimSpace(s), "SHA256:")
	if decoded, err := base64.StdEncoding.DecodeString(s); err == nil && len(decoded) == sha256.Size {
		return base64.StdEncoding.EncodeToString(decoded)
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(s); err == nil && len(decoded) == sha256.Size {
		return base64.StdEncoding.EncodeToString(decoded)
	}
	return strings.TrimSpace(s)
}

func decodeBase64Text(s string) (string, bool) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(s)
	}
	if err != nil || len(decoded) == 0 {
		return "", false
	}
	for _, b := range decoded {
		if b == '\n' || b == '\r' || b == '\t' {
			continue
		}
		if b < 0x20 || b > 0x7e {
			return "", false
		}
	}
	text := strings.TrimSpace(string(decoded))
	if strings.Contains(text, "SHA256:") || strings.HasPrefix(text, "256 ") {
		return text, true
	}
	return "", false
}

var secretLineRe = regexp.MustCompile(`(?i)(PASSWORD|TOKEN|PRIVATE_KEY|PASSPHRASE|AUTH_HEADER)=.*`)

func redactProvisionOutput(raw string) []string {
	lines := make([]string, 0, nodeProvisionOutputLimit)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if secretLineRe.MatchString(line) {
			parts := strings.SplitN(line, "=", 2)
			line = parts[0] + "=<redacted>"
		}
		lines = append(lines, line)
		if len(lines) > nodeProvisionOutputLimit {
			lines = lines[len(lines)-nodeProvisionOutputLimit:]
		}
	}
	return lines
}
