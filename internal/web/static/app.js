(() => {
  const phoneQueue = [];
  const queuedPhones = new WeakSet();
  let phoneWorkers = 0;
  let nextPageURL = document.getElementById("load-more")?.dataset.next || "";
  let preloadedPage = null;
  let preloadPromise = null;
  let appendInFlight = false;
  const previewCache = new Map();
  let previewBox = null;
  let previewTimer = 0;
  let previewTarget = null;
  let previewAbort = null;
  let previewHideTimer = 0;

  function boolValue(value) {
    return value === true || value === "true" || value === "1";
  }

  function escapeHTML(value) {
    const span = document.createElement("span");
    span.textContent = value || "";
    return span.innerHTML;
  }

  function updateShownCount() {
    const shown = document.getElementById("shown-count");
    const body = document.getElementById("ads-body");
    if (shown && body) shown.textContent = String(body.querySelectorAll("tr:not([data-empty-row])").length);
  }

  function renderPhone(cell, phone, calledBefore) {
    if (!cell) return;
    phone = phone || "Unavailable";
    delete cell.dataset.phoneSlug;
    cell.dataset.phoneValue = phone !== "Unavailable" ? phone : "";
    if (phone === "Unavailable") {
      cell.className = "muted";
      cell.textContent = "Unavailable";
      return;
    }
    cell.className = "phonecell";
    cell.innerHTML = `${calledBefore ? '<span class="calltag">Called before</span>' : ""}<span class="phone">${escapeHTML(phone)}</span>`;
  }

  function applyCalledState(row, called) {
    if (!row) return;
    row.classList.toggle("is-called", called);
    const input = row.querySelector("[data-called-slug]");
    if (input) input.checked = called;
  }

  function removeIfFilteredOut(row, called) {
    const mode = new URLSearchParams(window.location.search).get("called") || "";
    if ((mode === "hide" && called) || (mode === "only" && !called)) {
      row.remove();
      updateShownCount();
    }
  }

  async function load(cell) {
    const slug = cell.dataset.phoneSlug;
    if (!slug) return;
    try {
      const res = await fetch(`/api/phone/${encodeURIComponent(slug)}`, {headers: {"Accept": "application/json"}});
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      renderPhone(cell, data.phone || "Unavailable", boolValue(data.calledBefore));
      applyCalledState(cell.closest("tr"), boolValue(data.called));
    } catch {
      renderPhone(cell, "Unavailable", false);
    }
  }

  function queuePhones(root = document) {
    for (const cell of root.querySelectorAll("[data-phone-slug]")) {
      if (queuedPhones.has(cell)) continue;
      queuedPhones.add(cell);
      phoneQueue.push(cell);
    }
    startPhoneWorkers();
  }

  function startPhoneWorkers() {
    while (phoneWorkers < 3 && phoneQueue.length > 0) {
      phoneWorkers++;
      phoneWorker().finally(() => {
        phoneWorkers--;
        startPhoneWorkers();
      });
    }
  }

  async function phoneWorker() {
    while (phoneQueue.length > 0) {
      const cell = phoneQueue.shift();
      await load(cell);
    }
  }

  async function fetchNextPage(url) {
    if (!url) return null;
    const res = await fetch(url, {headers: {"Accept": "application/json"}});
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return await res.json();
  }

  function preloadNextPage() {
    if (!nextPageURL || preloadPromise || preloadedPage) return;
    const url = nextPageURL;
    preloadPromise = fetchNextPage(url)
      .then((data) => { preloadedPage = data; })
      .catch(() => {})
      .finally(() => { preloadPromise = null; });
  }

  async function appendNextPage() {
    const marker = document.getElementById("load-more");
    const body = document.getElementById("ads-body");
    if (!marker || !body || marker.classList.contains("done") || appendInFlight) return;

    appendInFlight = true;
    marker.classList.remove("error");
    marker.lastElementChild.textContent = "Loading more ads";
    try {
      let data = preloadedPage;
      preloadedPage = null;
      if (!data) {
        if (preloadPromise) await preloadPromise;
        data = preloadedPage;
        preloadedPage = null;
      }
      if (!data) data = await fetchNextPage(nextPageURL);
      if (!data || data.count === 0 || !data.rows) {
        marker.classList.add("done");
        marker.lastElementChild.textContent = "No more ads";
        return;
      }
      nextPageURL = data.next || "";
      body.insertAdjacentHTML("beforeend", data.rows);
      updateShownCount();
      queuePhones(body);
      marker.dataset.next = nextPageURL;
      if (Array.isArray(data.skipped) && data.skipped.length) {
        marker.lastElementChild.textContent = `Skipped unavailable page ${data.skipped.join(", ")}`;
      }
      preloadNextPage();
    } catch {
      marker.classList.add("error");
      marker.lastElementChild.textContent = "Load paused. Scroll again to retry.";
      preloadPromise = null;
    } finally {
      appendInFlight = false;
    }
  }

  function ensurePreviewBox() {
    if (previewBox) return previewBox;
    previewBox = document.createElement("div");
    previewBox.className = "hover-preview";
    previewBox.hidden = true;
    previewBox.addEventListener("mouseenter", () => clearTimeout(previewHideTimer));
    previewBox.addEventListener("mouseleave", schedulePreviewHide);
    document.body.appendChild(previewBox);
    return previewBox;
  }

  function positionPreview(target) {
    const box = ensurePreviewBox();
    const rect = target.getBoundingClientRect();
    const gap = 8;
    const width = Math.min(540, window.innerWidth - 16);
    box.style.width = `${width}px`;
    let left = rect.right + gap;
    if (left + width > window.innerWidth - gap) {
      left = Math.max(gap, rect.left - width - gap);
    }
    let top = Math.min(rect.top, window.innerHeight - box.offsetHeight - gap);
    top = Math.max(gap, top);
    box.style.left = `${left}px`;
    box.style.top = `${top}px`;
  }

  function schedulePreviewHide() {
    clearTimeout(previewHideTimer);
    previewHideTimer = window.setTimeout(hidePreview, 160);
  }

  function hidePreview() {
    clearTimeout(previewTimer);
    previewTarget = null;
    if (previewAbort) previewAbort.abort();
    previewAbort = null;
    if (previewBox) previewBox.hidden = true;
  }

  async function showPreview(target) {
    const slug = target.dataset.previewSlug;
    if (!slug) return;
    clearTimeout(previewHideTimer);
    previewTarget = target;
    const box = ensurePreviewBox();
    box.hidden = false;
    box.innerHTML = previewCache.get(slug) || `<div class="preview-loading"><span class="spinner"></span><span>Loading details</span></div>`;
    positionPreview(target);

    if (previewCache.has(slug)) return;
    if (previewAbort) previewAbort.abort();
    previewAbort = new AbortController();
    try {
      const res = await fetch(`/api/preview/${encodeURIComponent(slug)}`, {
        headers: {"Accept": "text/html"},
        signal: previewAbort.signal,
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const html = await res.text();
      previewCache.set(slug, html);
      if (previewTarget === target) {
        box.innerHTML = html;
        positionPreview(target);
      }
    } catch (err) {
      if (err.name === "AbortError") return;
      if (previewTarget === target) {
        box.innerHTML = `<div class="preview-error">Could not load preview.</div>`;
        positionPreview(target);
      }
    } finally {
      previewAbort = null;
    }
  }

  function schedulePreview(target) {
    clearTimeout(previewTimer);
    clearTimeout(previewHideTimer);
    previewTimer = window.setTimeout(() => showPreview(target), 280);
  }

  function previewTriggerFrom(event) {
    return event.target instanceof Element ? event.target.closest("[data-preview-slug]") : null;
  }

  async function saveCalled(input) {
    const slug = input.dataset.calledSlug;
    if (!slug) return;
    const row = input.closest("tr");
    const phoneCell = row?.querySelector("[data-phone-value], [data-phone-slug]");
    const phone = phoneCell?.dataset.phoneValue || "";
    const title = input.dataset.calledTitle || row?.querySelector(".title")?.textContent || "";
    const requested = input.checked;
    input.disabled = true;
    try {
      const res = await fetch(`/api/called/${encodeURIComponent(slug)}`, {
        method: "POST",
        headers: {"Accept": "application/json", "Content-Type": "application/json"},
        body: JSON.stringify({called: requested, phone, title}),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      const called = boolValue(data.called);
      applyCalledState(row, called);
      if (data.phone) {
        renderPhone(phoneCell, data.phone, boolValue(data.calledBefore));
      }
      if (row) removeIfFilteredOut(row, called);
    } catch {
      input.checked = !requested;
    } finally {
      input.disabled = false;
    }
  }

  document.addEventListener("change", (event) => {
    const input = event.target instanceof Element ? event.target.closest("[data-called-slug]") : null;
    if (input) saveCalled(input);
  });

  document.addEventListener("mouseover", (event) => {
    const target = previewTriggerFrom(event);
    if (!target || target === previewTarget) return;
    schedulePreview(target);
  });

  document.addEventListener("mouseout", (event) => {
    const target = previewTriggerFrom(event);
    if (!target) return;
    const next = event.relatedTarget;
    if (next && (target.contains(next) || ensurePreviewBox().contains(next))) return;
    schedulePreviewHide();
  });

  document.addEventListener("focusin", (event) => {
    const target = previewTriggerFrom(event);
    if (target) schedulePreview(target);
  });

  document.addEventListener("focusout", (event) => {
    const target = previewTriggerFrom(event);
    if (target) schedulePreviewHide();
  });

  window.addEventListener("scroll", () => {
    if (previewTarget && previewBox && !previewBox.hidden) positionPreview(previewTarget);
  }, {passive: true});

  window.addEventListener("resize", () => {
    if (previewTarget && previewBox && !previewBox.hidden) positionPreview(previewTarget);
  });

  queuePhones();

  const marker = document.getElementById("load-more");
  if (marker) {
    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) appendNextPage();
    }, {rootMargin: "1400px 0px"});
    observer.observe(marker);
    preloadNextPage();
  }
})();
