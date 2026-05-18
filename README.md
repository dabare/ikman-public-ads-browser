# ikman public ads browser

A compact Go web app for browsing public ikman.lk listings in a table, filtering the current result page, previewing full ad details on hover, and opening detail pages with images, properties, seller contact data, and similar ads. Detail links keep the active table filters so Back to table returns to the same search.

## Disclaimer

This project is for learning and educational purposes only. It is an independent local demo for studying Go web apps, server-rendered UI, filtering, caching, and responsible handling of publicly visible web data.

This project is not affiliated with, endorsed by, or sponsored by ikman.lk. Use it responsibly and respect ikman.lk's terms, robots policies, rate limits, copyrights, and the privacy of advertisers. Do not use this project for scraping at scale, spam, automated contacting, resale of data, or any abusive activity.

The local called-ad tracker stores data only on your own machine in `data/calls.json` by default. Do not publish personal call history, phone numbers, or generated local data.

## Run

```sh
go run ./cmd/ikman-browser
```

Open `http://localhost:8080`.

## Build

```sh
./scripts/build.sh
```

Outputs are written to `dist/`:

- `ikman-browser-darwin-arm64`
- `ikman-browser-darwin-amd64`
- `ikman-browser-windows-amd64.exe`
- `ikman-browser-linux-amd64`

Build a single target with:

```sh
./scripts/build_macos.sh
./scripts/build_windows.sh
./scripts/build_linux.sh
```

Set `ARCH=arm64` or `ARCH=amd64` when needed.

## Configuration

- `PORT` - default `8080`
- `IKMAN_BASE_URL` - default `https://ikman.lk`
- `IKMAN_REQUEST_INTERVAL` - default `200ms`
- `IKMAN_LOAD_PHONES` - default `true`
- `IKMAN_CALL_DB` - default `data/calls.json`, stores locally marked called ads and phone numbers

The app uses public HTML pages and `window.initialData`; it does not call authenticated or protected mutation endpoints.

## Filters

Keyword, location, category, seller/shop, price range, ad type, min photos, called status, member, verified, featured, image-only, authorized dealer, doorstep delivery, free delivery, top, urgent, extra-photo, phone loading, and local sort options are available from the sidebar.
