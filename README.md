# Adblock Recovery Sink

Serves harmless replacements for known anti-adblock loader resources, so pages stay usable while network-level ad and tracker blocking remains active. Works for desktop, phone and tablet browsers without extensions.

The sink sits behind DNS rewrites that you manage on your LAN resolver (AdGuard Home, NextDNS, or similar). The application never changes DNS.

## Status

Scaffold only. The substitution approach is proven in [docs/compatibility.md](docs/compatibility.md); the service itself is not implemented yet.

## Trust model

Run your own instance and never enroll devices in someone else's. Installing an instance's root certificate lets its operator impersonate any website to your devices, and the sink delivers JavaScript that runs inside the pages you visit. Only trust a root you generated yourself.

## Prior art

- [tinyShield](https://github.com/FilteringDev/tinyShield)

## License

MIT
