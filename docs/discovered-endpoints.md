# Discovered ikman.lk public surfaces

This project intentionally uses only public pages that are already viewable without authentication.

Observed surfaces:

- `GET https://ikman.lk/en/ads` - public listing page. It embeds listing rows in `window.initialData.serp.ads.data.ads`.
- `GET https://ikman.lk/en/ads/{location}/{category}` - public listing variants, for example `/en/ads/colombo/mobile-phones`.
- `GET https://ikman.lk/en/ads?query={term}` - user-entered search query.
- `GET https://ikman.lk/en/ad/{slug}` - public detail page. It embeds details in `window.initialData.adDetail.data.ad`, including images, properties, seller contact card, and phone numbers present in the public page data.
- `GET https://i.ikman-st.com/{slug}/{image-id}/{width}/{height}/{mode}.jpg` - public image CDN URL pattern where `mode` is usually `cropped` or `fitted`.

The page also exposes `window.apiURL = "https://api.ikman.lk"`, but this app does not use private, authenticated, edit/delete/report, login, chat, or hidden phone reveal APIs.

`robots.txt` disallows broad crawling of several query/filter URL forms. Treat this app as a user-driven browser with caching and throttling, not as a bulk scraper.

