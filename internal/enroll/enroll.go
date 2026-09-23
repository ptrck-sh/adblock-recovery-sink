package enroll

import (
	"bytes"
	"encoding/pem"
	"html/template"
	"net/http"
	"strconv"

	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/pki"
)

type pageData struct {
	EnrollmentHost string
	Hosts          []string
	Fingerprint    string
}

func Handler(issuer *pki.Issuer, enrollmentHost string) http.Handler {
	if issuer == nil || issuer.Root() == nil || issuer.Intermediate() == nil {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusInternalServerError)
		})
	}
	root := issuer.Root()
	intermediate := issuer.Intermediate()
	rootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: root.Raw})
	intermediatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: intermediate.Raw})
	chainPEM := append(append([]byte{}, intermediatePEM...), rootPEM...)
	page, err := renderInstall(pageData{
		EnrollmentHost: enrollmentHost,
		Hosts:          intermediate.PermittedDNSDomains,
		Fingerprint:    pki.Fingerprint(root),
	})
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			writer.Header().Set("Allow", "GET, HEAD")
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		switch request.URL.Path {
		case "/ca.crt":
			writeDownload(writer, request, "application/x-x509-ca-cert", "adblock-recovery-sink-root.crt", root.Raw)
		case "/ca.pem":
			writeBody(writer, request, "application/x-pem-file", rootPEM)
		case "/ca-chain.pem":
			writeBody(writer, request, "application/x-pem-file", chainPEM)
		case "/fingerprint":
			writeBody(writer, request, "text/plain", []byte(pki.Fingerprint(root)+"\n"))
		case "/install":
			writer.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src data:")
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
			writeBody(writer, request, "text/html; charset=utf-8", page)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	})
}

func writeDownload(writer http.ResponseWriter, request *http.Request, contentType, filename string, body []byte) {
	writer.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	writeBody(writer, request, contentType, body)
}

func writeBody(writer http.ResponseWriter, request *http.Request, contentType string, body []byte) {
	writer.Header().Set("Content-Type", contentType)
	writer.Header().Set("Content-Length", stringLength(len(body)))
	writer.WriteHeader(http.StatusOK)
	if request.Method == http.MethodGet {
		_, _ = writer.Write(body)
	}
}

func stringLength(length int) string {
	return strconv.Itoa(length)
}

func renderInstall(data pageData) ([]byte, error) {
	page, err := template.New("install").Parse(installTemplate)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := page.Execute(&output, data); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

const installTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Adblock Recovery Sink enrollment</title>
<style>
:root { color-scheme: light; font-family: system-ui, sans-serif; background: #eff1f5; color: #4c4f69; }
body { margin: 0; padding: 2rem 1rem; }
main { max-width: 50rem; margin: auto; background: #e6e9ef; border: 1px solid #ccd0da; border-radius: .75rem; padding: 2rem; }
h1, h2 { color: #1e66f5; }
a { color: #1e66f5; }
.warning { background: #ccd0da; padding: 1rem; border-radius: .5rem; font-weight: 600; }
code { overflow-wrap: anywhere; }
li { margin: .45rem 0; }
</style>
</head>
<body>
<main>
<h1>Device enrollment</h1>
<p>This page provides the private root certificate for this Adblock Recovery Sink. It enables selected site recovery on this device.</p>
{{if .EnrollmentHost}}<p>Enrollment host: <code>{{.EnrollmentHost}}</code></p>{{end}}
<p class="warning">Only install a root you generated yourself; it lets its operator impersonate the listed hosts.</p>
<h2>Allowed hostnames</h2>
<ul>{{range .Hosts}}<li><code>{{.}}</code></li>{{end}}</ul>
<h2>Verify before installing</h2>
<p>Root SHA-256 fingerprint: <code>{{.Fingerprint}}</code></p>
<p><a href="/ca.crt">Download root certificate</a> · <a href="/ca.pem">Download PEM</a> · <a href="/ca-chain.pem">Download CA chain</a> · <a href="/fingerprint">Fingerprint text</a></p>
<h2>Install and remove</h2>
<ul>
<li><strong>Android:</strong> download the root certificate, then install it from Security settings as a CA certificate. Remove it from Trusted credentials when no longer needed.</li>
<li><strong>iOS/iPadOS:</strong> download the certificate, install the profile in Settings, then enable full trust in Certificate Trust Settings. Remove the profile in VPN and Device Management.</li>
<li><strong>Windows:</strong> import it into Local Computer, Trusted Root Certification Authorities. Remove that same certificate from the store to undo it.</li>
<li><strong>macOS:</strong> import it into the System keychain and set it to trust for SSL. Delete it from Keychain Access to remove it.</li>
<li><strong>Linux:</strong> add it to your distribution CA trust store and refresh that store. Remove the file and refresh the store to undo it.</li>
</ul>
<p>Firefox can use its own certificate store, so it may need a separate import. Chrome asks for Local Network Access per site.</p>
</main>
</body>
</html>`
