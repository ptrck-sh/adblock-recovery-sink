package pki

import (
	"container/list"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultRootValidity         = 10 * 365 * 24 * time.Hour
	defaultIntermediateValidity = 3 * 365 * 24 * time.Hour
	defaultLeafValidity         = 7 * 24 * time.Hour
	defaultCacheSize            = 256
)

type InitOptions struct {
	Hosts                []string
	OutDir               string
	Now                  func() time.Time
	RootValidity         time.Duration
	IntermediateValidity time.Duration
}

type IssuerOptions struct {
	Hosts        []string
	CacheSize    int
	LeafValidity time.Duration
	Now          func() time.Time
}

type Issuer struct {
	root         *x509.Certificate
	intermediate *x509.Certificate
	key          *ecdsa.PrivateKey
	hosts        map[string]struct{}
	now          func() time.Time
	leafValidity time.Duration
	cacheSize    int
	mu           sync.Mutex
	cache        map[string]*list.Element
	lru          *list.List
}

type cacheEntry struct {
	host string
	cert *tls.Certificate
}

func Init(opts InitOptions) error {
	hosts, err := normalizeHosts(opts.Hosts)
	if err != nil {
		return err
	}
	if opts.OutDir == "" {
		return errors.New("output directory is required")
	}
	rootValidity := opts.RootValidity
	if rootValidity <= 0 {
		rootValidity = defaultRootValidity
	}
	intermediateValidity := opts.IntermediateValidity
	if intermediateValidity <= 0 {
		intermediateValidity = defaultIntermediateValidity
	}
	now := clock(opts.Now)()
	if err := os.MkdirAll(opts.OutDir, 0700); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	paths := []string{
		filepath.Join(opts.OutDir, "root.crt"),
		filepath.Join(opts.OutDir, "root.key"),
		filepath.Join(opts.OutDir, "intermediate.crt"),
		filepath.Join(opts.OutDir, "intermediate.key"),
	}
	for _, path := range paths {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("refusing to overwrite %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect output %s: %w", path, err)
		}
	}

	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate root key: %w", err)
	}
	rootSKI, err := subjectKeyID(rootKey.Public())
	if err != nil {
		return fmt.Errorf("root subject key identifier: %w", err)
	}
	rootSerial, err := serialNumber()
	if err != nil {
		return err
	}
	rootTemplate := &x509.Certificate{
		SerialNumber:                rootSerial,
		Subject:                     pkix.Name{CommonName: "adblock-recovery-sink root"},
		NotBefore:                   now,
		NotAfter:                    now.Add(rootValidity),
		IsCA:                        true,
		BasicConstraintsValid:       true,
		MaxPathLen:                  1,
		KeyUsage:                    x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		SubjectKeyId:                rootSKI,
		AuthorityKeyId:              rootSKI,
		PermittedDNSDomains:         hosts,
		PermittedDNSDomainsCritical: true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, rootKey.Public(), rootKey)
	if err != nil {
		return fmt.Errorf("create root certificate: %w", err)
	}
	rootCert, err := x509.ParseCertificate(rootDER)
	if err != nil {
		return fmt.Errorf("parse root certificate: %w", err)
	}

	intermediateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate intermediate key: %w", err)
	}
	intermediateSKI, err := subjectKeyID(intermediateKey.Public())
	if err != nil {
		return fmt.Errorf("intermediate subject key identifier: %w", err)
	}
	intermediateSerial, err := serialNumber()
	if err != nil {
		return err
	}
	intermediateNotAfter := now.Add(intermediateValidity)
	if intermediateNotAfter.After(rootCert.NotAfter) {
		intermediateNotAfter = rootCert.NotAfter
	}
	intermediateTemplate := &x509.Certificate{
		SerialNumber:                intermediateSerial,
		Subject:                     pkix.Name{CommonName: "adblock-recovery-sink intermediate"},
		NotBefore:                   now,
		NotAfter:                    intermediateNotAfter,
		IsCA:                        true,
		BasicConstraintsValid:       true,
		MaxPathLenZero:              true,
		KeyUsage:                    x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		SubjectKeyId:                intermediateSKI,
		AuthorityKeyId:              rootCert.SubjectKeyId,
		PermittedDNSDomains:         hosts,
		PermittedDNSDomainsCritical: true,
	}
	intermediateDER, err := x509.CreateCertificate(rand.Reader, intermediateTemplate, rootCert, intermediateKey.Public(), rootKey)
	if err != nil {
		return fmt.Errorf("create intermediate certificate: %w", err)
	}
	rootKeyPEM, err := privateKeyPEM(rootKey)
	if err != nil {
		return fmt.Errorf("encode root key: %w", err)
	}
	intermediateKeyPEM, err := privateKeyPEM(intermediateKey)
	if err != nil {
		return fmt.Errorf("encode intermediate key: %w", err)
	}
	files := []struct {
		path string
		data []byte
	}{
		{filepath.Join(opts.OutDir, "root.crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER})},
		{filepath.Join(opts.OutDir, "root.key"), rootKeyPEM},
		{filepath.Join(opts.OutDir, "intermediate.crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: intermediateDER})},
		{filepath.Join(opts.OutDir, "intermediate.key"), intermediateKeyPEM},
	}
	created := make([]string, 0, len(files))
	for _, file := range files {
		if err := writeNew(file.path, file.data); err != nil {
			for _, path := range created {
				_ = os.Remove(path)
			}
			return err
		}
		created = append(created, file.path)
	}
	return nil
}

func RunInit(args []string, stdout io.Writer) error {
	if len(args) > 0 && args[0] == "init" {
		args = args[1:]
	}
	flags := flag.NewFlagSet("pki init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	hosts := flags.String("hosts", "", "")
	out := flags.String("out", "", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	if stdout == nil {
		return errors.New("stdout is required")
	}
	if err := Init(InitOptions{
		Hosts:  strings.Split(*hosts, ","),
		OutDir: *out,
	}); err != nil {
		return err
	}
	rootPEM, err := os.ReadFile(filepath.Join(*out, "root.crt"))
	if err != nil {
		return fmt.Errorf("read root certificate: %w", err)
	}
	root, err := parseCertificate(rootPEM, "root certificate")
	if err != nil {
		return err
	}
	for _, name := range []string{"root.crt", "root.key", "intermediate.crt", "intermediate.key"} {
		if _, err := fmt.Fprintln(stdout, filepath.Join(*out, name)); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(stdout, "root SHA-256 fingerprint: %s\n", Fingerprint(root))
	return err
}

func Load(rootPEM, intermediatePEM, intermediateKeyPEM []byte, opts IssuerOptions) (*Issuer, error) {
	hosts, err := normalizeHosts(opts.Hosts)
	if err != nil {
		return nil, err
	}
	root, err := parseCertificate(rootPEM, "root certificate")
	if err != nil {
		return nil, err
	}
	intermediate, err := parseCertificate(intermediatePEM, "intermediate certificate")
	if err != nil {
		return nil, err
	}
	key, err := parseECDSAKey(intermediateKeyPEM)
	if err != nil {
		return nil, err
	}
	if !publicKeysEqual(key.Public(), intermediate.PublicKey) {
		return nil, errors.New("intermediate key does not match intermediate certificate")
	}
	if !validCA(root) {
		return nil, errors.New("root certificate is not a valid CA")
	}
	if !validCA(intermediate) {
		return nil, errors.New("intermediate certificate is not a valid CA")
	}
	if intermediate.NotAfter.After(root.NotAfter) {
		return nil, errors.New("intermediate certificate outlives root certificate")
	}
	now := clock(opts.Now)
	current := now()
	if err := validAt(root, current); err != nil {
		return nil, fmt.Errorf("root certificate: %w", err)
	}
	if err := validAt(intermediate, current); err != nil {
		return nil, fmt.Errorf("intermediate certificate: %w", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(root)
	if _, err := intermediate.Verify(x509.VerifyOptions{
		Roots:       pool,
		CurrentTime: current,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return nil, fmt.Errorf("intermediate does not chain to root: %w", err)
	}
	for _, host := range hosts {
		if !hostAllowedByCertificate(root, host) {
			return nil, fmt.Errorf("host %q is outside root name constraints", host)
		}
		if !hostAllowedByCertificate(intermediate, host) {
			return nil, fmt.Errorf("host %q is outside intermediate name constraints", host)
		}
	}
	cacheSize := opts.CacheSize
	if cacheSize <= 0 {
		cacheSize = defaultCacheSize
	}
	leafValidity := opts.LeafValidity
	if leafValidity <= 0 {
		leafValidity = defaultLeafValidity
	}
	hostSet := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		hostSet[host] = struct{}{}
	}
	return &Issuer{
		root:         root,
		intermediate: intermediate,
		key:          key,
		hosts:        hostSet,
		now:          now,
		leafValidity: leafValidity,
		cacheSize:    cacheSize,
		cache:        make(map[string]*list.Element),
		lru:          list.New(),
	}, nil
}

func (i *Issuer) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if hello == nil {
		return nil, errors.New("client hello is required")
	}
	host, err := normalizeHost(hello.ServerName)
	if err != nil {
		return nil, fmt.Errorf("invalid SNI: %w", err)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if _, ok := i.hosts[host]; !ok {
		return nil, fmt.Errorf("SNI %q is not configured", host)
	}
	now := i.now()
	if err := i.readyAt(now); err != nil {
		return nil, err
	}
	if element, ok := i.cache[host]; ok {
		entry := element.Value.(*cacheEntry)
		if !needsRenewal(entry.cert, now) {
			i.lru.MoveToFront(element)
			return entry.cert, nil
		}
		i.lru.Remove(element)
		delete(i.cache, host)
	}
	cert, err := i.issue(host, now)
	if err != nil {
		return nil, err
	}
	element := i.lru.PushFront(&cacheEntry{host: host, cert: cert})
	i.cache[host] = element
	if i.lru.Len() > i.cacheSize {
		oldest := i.lru.Back()
		entry := oldest.Value.(*cacheEntry)
		delete(i.cache, entry.host)
		i.lru.Remove(oldest)
	}
	return cert, nil
}

func (i *Issuer) Ready() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.readyAt(i.now())
}

func (i *Issuer) CacheLen() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.lru.Len()
}

func (i *Issuer) ExpiresAt() time.Time {
	if i.root.NotAfter.Before(i.intermediate.NotAfter) {
		return i.root.NotAfter
	}
	return i.intermediate.NotAfter
}

func (i *Issuer) Root() *x509.Certificate {
	return i.root
}

func (i *Issuer) Intermediate() *x509.Certificate {
	return i.intermediate
}

func Fingerprint(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	sum := sha256.Sum256(cert.Raw)
	parts := make([]string, len(sum))
	for index, value := range sum {
		parts[index] = strings.ToUpper(hex.EncodeToString([]byte{value}))
	}
	return strings.Join(parts, ":")
}

func (i *Issuer) issue(host string, now time.Time) (*tls.Certificate, error) {
	serial, err := serialNumber()
	if err != nil {
		return nil, err
	}
	notAfter := now.Add(i.leafValidity)
	if notAfter.After(i.intermediate.NotAfter) {
		notAfter = i.intermediate.NotAfter
	}
	if notAfter.After(i.root.NotAfter) {
		notAfter = i.root.NotAfter
	}
	if !notAfter.After(now) {
		return nil, errors.New("issuer certificate has expired")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate leaf key: %w", err)
	}
	ski, err := subjectKeyID(key.Public())
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: host},
		DNSNames:              []string{host},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		SubjectKeyId:          ski,
		AuthorityKeyId:        i.intermediate.SubjectKeyId,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, i.intermediate, key.Public(), i.key)
	if err != nil {
		return nil, fmt.Errorf("create leaf certificate: %w", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse leaf certificate: %w", err)
	}
	return &tls.Certificate{
		Certificate: [][]byte{der, i.intermediate.Raw},
		PrivateKey:  key,
		Leaf:        leaf,
	}, nil
}

func (i *Issuer) readyAt(now time.Time) error {
	if err := validAt(i.root, now); err != nil {
		return fmt.Errorf("root certificate: %w", err)
	}
	if err := validAt(i.intermediate, now); err != nil {
		return fmt.Errorf("intermediate certificate: %w", err)
	}
	return nil
}

func needsRenewal(cert *tls.Certificate, now time.Time) bool {
	if cert == nil || cert.Leaf == nil {
		return true
	}
	lifetime := cert.Leaf.NotAfter.Sub(cert.Leaf.NotBefore)
	return cert.Leaf.NotAfter.Sub(now) < lifetime/3
}

func normalizeHosts(hosts []string) ([]string, error) {
	if len(hosts) == 0 {
		return nil, errors.New("at least one host is required")
	}
	result := make([]string, 0, len(hosts))
	seen := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		normalized, err := normalizeHost(host)
		if err != nil {
			return nil, fmt.Errorf("invalid host %q: %w", host, err)
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result, nil
}

func normalizeHost(host string) (string, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimSuffix(host, ".")
	if host == "" || len(host) > 253 || strings.ContainsAny(host, "*/:\\") {
		return "", errors.New("must be an exact DNS hostname")
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New("must be an exact DNS hostname")
		}
		for _, character := range label {
			if !(character >= 'a' && character <= 'z') && !(character >= '0' && character <= '9') && character != '-' {
				return "", errors.New("must be an exact DNS hostname")
			}
		}
	}
	return host, nil
}

func hostAllowedByCertificate(cert *x509.Certificate, host string) bool {
	if len(cert.PermittedDNSDomains) > 0 {
		permitted := false
		for _, domain := range cert.PermittedDNSDomains {
			if domainMatches(domain, host) {
				permitted = true
				break
			}
		}
		if !permitted {
			return false
		}
	}
	for _, domain := range cert.ExcludedDNSDomains {
		if domainMatches(domain, host) {
			return false
		}
	}
	return true
}

func domainMatches(domain, host string) bool {
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	if strings.HasPrefix(domain, ".") {
		return strings.HasSuffix(host, domain) && host != strings.TrimPrefix(domain, ".")
	}
	return host == domain || strings.HasSuffix(host, "."+domain)
}

func parseCertificate(data []byte, description string) (*x509.Certificate, error) {
	blocks, err := decodePEM(data, description)
	if err != nil {
		return nil, err
	}
	if len(blocks) != 1 || blocks[0].Type != "CERTIFICATE" {
		return nil, fmt.Errorf("%s must contain one CERTIFICATE PEM block", description)
	}
	cert, err := x509.ParseCertificate(blocks[0].Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", description, err)
	}
	return cert, nil
}

func parseECDSAKey(data []byte) (*ecdsa.PrivateKey, error) {
	blocks, err := decodePEM(data, "intermediate key")
	if err != nil {
		return nil, err
	}
	if len(blocks) != 1 {
		return nil, errors.New("intermediate key must contain one PEM block")
	}
	var key any
	switch blocks[0].Type {
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(blocks[0].Bytes)
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(blocks[0].Bytes)
	default:
		return nil, errors.New("intermediate key must be an ECDSA private key")
	}
	if err != nil {
		return nil, fmt.Errorf("parse intermediate key: %w", err)
	}
	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("intermediate key must be an ECDSA private key")
	}
	return ecdsaKey, nil
}

func decodePEM(data []byte, description string) ([]*pem.Block, error) {
	var blocks []*pem.Block
	for len(strings.TrimSpace(string(data))) > 0 {
		block, rest := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("invalid %s PEM", description)
		}
		blocks = append(blocks, block)
		data = rest
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("invalid %s PEM", description)
	}
	return blocks, nil
}

func validAt(cert *x509.Certificate, now time.Time) error {
	if now.Before(cert.NotBefore) {
		return errors.New("is not yet valid")
	}
	if !now.Before(cert.NotAfter) {
		return errors.New("has expired")
	}
	return nil
}

func validCA(cert *x509.Certificate) bool {
	return cert.IsCA && cert.BasicConstraintsValid && cert.KeyUsage&x509.KeyUsageCertSign != 0
}

func serialNumber() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("generate serial number: %w", err)
	}
	return serial, nil
}

func subjectKeyID(public crypto.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(der)
	return sum[:], nil
}

func privateKeyPEM(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

func publicKeysEqual(left, right crypto.PublicKey) bool {
	leftDER, leftErr := x509.MarshalPKIXPublicKey(left)
	rightDER, rightErr := x509.MarshalPKIXPublicKey(right)
	return leftErr == nil && rightErr == nil && string(leftDER) == string(rightDER)
}

func writeNew(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}

func clock(now func() time.Time) func() time.Time {
	if now != nil {
		return now
	}
	return time.Now
}
