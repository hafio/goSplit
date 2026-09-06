// Live sync for the currency conversion page. The from-amount is the anchor:
// editing it or the rate recomputes the to-amount; editing the to-amount
// recomputes the rate. Rate provenance (fetched vs custom) is shown. Everything
// here is progressive — with JS off, the user types both amounts and the server
// stores them verbatim.
(function () {
  "use strict";
  var form, from, to, rate, src, fromCur, toCur, dir, summary, dateEl;

  // Writing .value alone leaves defaultValue behind, and the background-refresh
  // guard reads that gap as "the user typed something" -- which would exempt
  // this page from refreshing forever, since the rate is filled in on load.
  // These writes are derived state, not edits, so move both.
  function setValue(el, v) { el.value = v; el.defaultValue = v; }

  var ZERO_DECIMAL = { JPY: 1, KRW: 1, VND: 1, CLP: 1, ISK: 1 };
  function dec(cur) { return ZERO_DECIMAL[cur] ? 0 : 2; }
  function num(el) { var v = parseFloat((el.value || "").replace(/[^0-9.]/g, "")); return isFinite(v) ? v : 0; }
  function sig6(x) { return Number(x.toPrecision(6)).toString(); }
  function fmt(x, cur) { return x.toFixed(dec(cur)); }

  function labels() {
    document.getElementById("cvRateFrom").textContent = fromCur.value;
    document.getElementById("cvRateTo").textContent = toCur.value;
  }
  function who() {
    var c = form.querySelector('input[name="direction"]:checked');
    return c ? c.closest("label").textContent.trim() : "";
  }
  function setSrc(custom) {
    src.textContent = custom ? src.getAttribute("data-custom") : src.getAttribute("data-fetched");
    src.className = "rate-src " + (custom ? "custom" : "fetched");
  }
  function summarize() {
    summary.textContent = fmt(num(from), fromCur.value) + " " + fromCur.value + " → " +
      fmt(num(to), toCur.value) + " " + toCur.value + "  (" + who() + ")";
  }
  function recomputeTo() {
    var r = num(rate);
    if (num(from) > 0 && r > 0) setValue(to, fmt(num(from) * r, toCur.value));
    summarize();
  }
  function fetchRate() {
    var f = fromCur.value, t = toCur.value;
    if (f === t) { setValue(rate, "1"); setSrc(false); recomputeTo(); return; }
    var d = dateEl ? dateEl.value : "";
    fetch("/rates?from=" + encodeURIComponent(f) + "&to=" + encodeURIComponent(t) + "&date=" + encodeURIComponent(d))
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (j) {
        if (j && j.rate) { setValue(rate, sig6(parseFloat(j.rate))); setSrc(false); recomputeTo(); }
      })
      .catch(function () { /* keep whatever's there; user can type a rate */ });
  }

  // Re-wire on htmx:load as well as at parse time: a boosted navigation does not
  // re-run this script when the tag is unchanged, and a morphed swap can replace
  // these controls with listener-less copies. The WeakSet makes that idempotent
  // without a DOM marker, which a morph would strip back off.
  var wired = new WeakSet();
  function wireOnce(el, fn) {
    if (!el || wired.has(el)) return;
    wired.add(el);
    fn(el);
  }

  function wire() {
    form = document.getElementById("convert-form");
    if (!form) return;
    from = document.getElementById("cvFrom");
    to = document.getElementById("cvTo");
    rate = document.getElementById("cvRate");
    src = document.getElementById("cvSrc");
    fromCur = document.getElementById("cvFromCur");
    toCur = document.getElementById("cvToCur");
    dir = document.getElementById("cvDir");
    summary = document.getElementById("cvSummary");
    dateEl = form.querySelector('input[name="date"]');

    wireOnce(from, function (el) { el.addEventListener("input", recomputeTo); });
    wireOnce(rate, function (el) {
      el.addEventListener("input", function () { setSrc(true); recomputeTo(); });
    });
    wireOnce(to, function (el) {
      el.addEventListener("input", function () {
        var f = num(from), t = num(to);
        if (f > 0 && t > 0) { setValue(rate, sig6(t / f)); setSrc(true); }
        summarize();
      });
    });
    wireOnce(fromCur, function (el) { el.addEventListener("change", function () { labels(); fetchRate(); }); });
    wireOnce(toCur, function (el) { el.addEventListener("change", function () { labels(); fetchRate(); }); });
    wireOnce(document.getElementById("cvRefresh"), function (el) { el.addEventListener("click", fetchRate); });
    wireOnce(dir, function (el) { el.addEventListener("change", summarize); });

    form.querySelectorAll("#cvChips .chip").forEach(function (chip) {
      wireOnce(chip, function (el) {
        el.addEventListener("click", function () {
          // A chip click is a real edit, so these deliberately move .value only:
          // the page should read as dirty afterwards.
          fromCur.value = el.getAttribute("data-cur");
          from.value = el.getAttribute("data-amt");
          var d = el.getAttribute("data-dir");
          var radio = form.querySelector('input[name="direction"][value="' + d + '"]');
          if (radio) radio.checked = true;
          if (toCur.value === fromCur.value) toCur.value = fromCur.value === "USD" ? "SGD" : "USD";
          labels(); fetchRate();
        });
      });
    });

    labels();
    fetchRate();
  }

  wire();
  document.addEventListener("htmx:load", wire);
})();
