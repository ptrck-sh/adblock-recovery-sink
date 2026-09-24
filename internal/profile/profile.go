package profile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"gitlab.com/ptrck-sh/adblock-recovery-sink/profiles"
)

const maxBody = 1 << 20

type Profile struct {
	Version int
	Name    string
	Routes  []Route
}

type Route struct {
	Profile      string
	Hosts        []string
	Path         string
	Methods      []string
	Status       int
	ContentType  string
	CacheControl string
	CORSOrigin   string
	Body         []byte
}

type Result string

const (
	Matched          Result = "matched"
	UnknownHost      Result = "unknown_host"
	UnknownPath      Result = "unknown_path"
	MethodNotAllowed Result = "method_not_allowed"
)

type Router struct {
	routes []Route
}

func Load(operatorDir string) (map[string]Profile, error) {
	bundled, err := loadFS(profiles.FS)
	if err != nil {
		return nil, err
	}
	if operatorDir == "" {
		return bundled, nil
	}
	overrides, err := loadFS(os.DirFS(operatorDir))
	if err != nil {
		return nil, err
	}
	for name, item := range overrides {
		bundled[name] = item
	}
	return bundled, nil
}

func loadFS(source fs.FS) (map[string]Profile, error) {
	items := map[string]Profile{}
	err := fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || path.Base(name) != "profile.yaml" {
			return nil
		}
		item, err := parse(source, name)
		if err != nil {
			return err
		}
		if _, exists := items[item.Name]; exists {
			return fmt.Errorf("duplicate profile %q", item.Name)
		}
		items[item.Name] = item
		return nil
	})
	return items, err
}

func parse(source fs.FS, file string) (Profile, error) {
	data, err := fs.ReadFile(source, file)
	if err != nil {
		return Profile{}, err
	}
	value, err := yaml.Parser().Unmarshal(data)
	if err != nil {
		return Profile{}, err
	}
	item := Profile{Version: number(value["version"]), Name: text(value["name"])}
	if item.Version != 1 || item.Name == "" {
		return Profile{}, errors.New("invalid profile version or name")
	}
	rawRoutes, ok := value["routes"].([]interface{})
	if !ok || len(rawRoutes) == 0 {
		return Profile{}, errors.New("profile has no routes")
	}
	dir := path.Dir(file)
	for _, raw := range rawRoutes {
		fields, ok := raw.(map[string]interface{})
		if !ok {
			return Profile{}, errors.New("invalid route")
		}
		route := Route{Profile: item.Name, Hosts: stringsOf(fields["hosts"]), Path: text(fields["path"]), Methods: stringsOf(fields["methods"]), Status: number(fields["status"]), ContentType: text(fields["content_type"]), CacheControl: text(fields["cache_control"]), CORSOrigin: text(fields["cors_origin"])}
		bodyFile := text(fields["body_file"])
		if err := validateRoute(&route, bodyFile); err != nil {
			return Profile{}, err
		}
		bodyPath := path.Clean(path.Join(dir, bodyFile))
		if bodyPath != path.Join(dir, bodyFile) || !strings.HasPrefix(bodyPath, dir+"/") {
			return Profile{}, errors.New("invalid body file")
		}
		route.Body, err = fs.ReadFile(source, bodyPath)
		if err != nil || len(route.Body) > maxBody {
			return Profile{}, errors.New("invalid body file")
		}
		item.Routes = append(item.Routes, route)
	}
	return item, nil
}

func validateRoute(route *Route, bodyFile string) error {
	if !strings.HasPrefix(route.Path, "/") || route.Path == "" || bodyFile == "" || path.IsAbs(bodyFile) {
		return errors.New("invalid route path or body")
	}
	if len(route.Hosts) == 0 || len(route.Methods) == 0 || route.Status < 100 || route.Status > 599 {
		return errors.New("invalid route")
	}
	for i, host := range route.Hosts {
		normalized, err := NormalizeHost(host)
		if err != nil || normalized != host || strings.Count(host, "*.") > 1 || (strings.Contains(host, "*") && !strings.HasPrefix(host, "*.")) {
			return errors.New("invalid route host")
		}
		route.Hosts[i] = normalized
	}
	for _, method := range route.Methods {
		if method != "GET" && method != "HEAD" && method != "POST" && method != "OPTIONS" {
			return errors.New("invalid route method")
		}
	}
	return nil
}

func NewRouter(items map[string]Profile, enabled []string) (*Router, error) {
	router := &Router{}
	for _, name := range enabled {
		item, ok := items[name]
		if !ok {
			return nil, fmt.Errorf("unknown profile %q", name)
		}
		router.routes = append(router.routes, item.Routes...)
	}
	return router, nil
}

func (r *Router) Match(host, method, requestPath string) (*Route, Result) {
	var pathRoute *Route
	for i := range r.routes {
		route := &r.routes[i]
		if !routeHasHost(route, host) {
			continue
		}
		if route.Path != requestPath {
			continue
		}
		pathRoute = route
		for _, allowed := range route.Methods {
			if allowed == method {
				return route, Matched
			}
		}
	}
	if pathRoute != nil {
		return pathRoute, MethodNotAllowed
	}
	for i := range r.routes {
		if routeHasHost(&r.routes[i], host) {
			return nil, UnknownPath
		}
	}
	return nil, UnknownHost
}

func (r *Router) Hosts() []string {
	set := map[string]bool{}
	for _, route := range r.routes {
		for _, host := range route.Hosts {
			set[host] = true
		}
	}
	result := make([]string, 0, len(set))
	for host := range set {
		result = append(result, host)
	}
	sort.Strings(result)
	return result
}

func RouteMatchesHost(route Route, host string) bool { return routeHasHost(&route, host) }

func ProfileMatchesHost(item Profile, host string) bool {
	for _, route := range item.Routes {
		if routeHasHost(&route, host) {
			return true
		}
	}
	return false
}

func NormalizeHost(host string) (string, error) {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return "", errors.New("empty host")
	}
	for _, char := range host {
		if char > 127 {
			return "", errors.New("non-ascii host")
		}
	}
	return host, nil
}

func routeHasHost(route *Route, host string) bool {
	for _, pattern := range route.Hosts {
		if pattern == host {
			return true
		}
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.TrimPrefix(pattern, "*.")
			if strings.HasSuffix(host, "."+suffix) {
				return true
			}
		}
	}
	return false
}

func text(value interface{}) string { result, _ := value.(string); return result }
func number(value interface{}) int {
	switch value := value.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}
func stringsOf(value interface{}) []string {
	raw, _ := value.([]interface{})
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if item, ok := item.(string); ok {
			result = append(result, item)
		}
	}
	return result
}
