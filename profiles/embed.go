package profiles

import "embed"

//go:embed adshield/profile.yaml adshield/loader.min.js adshield-scripts/profile.yaml adshield-scripts/loader.min.js toast.js
var FS embed.FS
