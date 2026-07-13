// Live sync for the currency conversion page. The from-amount is the anchor:
// editing it or the rate recomputes the to-amount; editing the to-amount
// recomputes the rate. Rate provenance (fetched vs custom) is shown. Everything
// here is progressive — with JS off, the user types both amounts and the server
// stores them verbatim.
(function () {
  "use strict";
  var form = document.getElementById("convert-form");
  if (!form) return;

  var from = document.getElementById("cvFrom"),
      to = document.getElementById("cvTo"),
      rate = document.getElementById("cvRate"),
      src = document.getElementById("cvSrc"),
      fromCur = document.getElementById("cvFromCur"),
      toCur = document.getElementById("cvToCur"),
      dir = document.getElementById("cvDir"),
      summary = document.getElementById("cvSummary"),
      dateEl = form.querySelector('input[name="date"]');

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
    if (num(from) > 0 && r > 0) to.value = fmt(num(from) * r, toCur.value);
    summarize();
  }
  function fetchRate() {
    var f = fromCur.value, t = toCur.value;
    if (f === t) { rate.value = "1"; setSrc(false); recomputeTo(); return; }
    var d = dateEl ? dateEl.value : "";
    fetch("/rates?from=" + encodeURIComponent(f) + "&to=" + encodeURIComponent(t) + "&date=" + encodeURIComponent(d))
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (j) {
        if (j && j.rate) { rate.value = sig6(parseFloat(j.rate)); setSrc(false); recomputeTo(); }
      })
      .catch(function () { /* keep whatever's there; user can type a rate */ });
  }

  from.addEventListener("input", recomputeTo);
  rate.addEventListener("input", function () { setSrc(true); recomputeTo(); });
  to.addEventListener("input", function () {
    var f = num(from), t = num(to);
    if (f > 0 && t > 0) { rate.value = sig6(t / f); setSrc(true); }
    summarize();
  });
  fromCur.addEventListener("change", function () { labels(); fetchRate(); });
  toCur.addEventListener("change", function () { labels(); fetchRate(); });
  document.getElementById("cvRefresh").addEventListener("click", fetchRate);
  dir.addEventListener("change", summarize);

  form.querySelectorAll("#cvChips .chip").forEach(function (chip) {
    chip.addEventListener("click", function () {
      fromCur.value = chip.getAttribute("data-cur");
      from.value = chip.getAttribute("data-amt");
      var d = chip.getAttribute("data-dir");
      var radio = form.querySelector('input[name="direction"][value="' + d + '"]');
      if (radio) radio.checked = true;
      if (toCur.value === fromCur.value) toCur.value = fromCur.value === "USD" ? "SGD" : "USD";
      labels(); fetchRate();
    });
  });

  labels();
  fetchRate();
})();
