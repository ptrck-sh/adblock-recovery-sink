package profiles

import "embed"

//go:embed adshield/profile.yaml adshield/loader.min.js toast.js
var FS embed.FS
