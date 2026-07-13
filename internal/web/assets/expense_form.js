// Progressive enhancement for the add/move/settle expense form. Everything here
// is cosmetic — the form submits and validates server-side without JS. It keeps
// the picker summaries in sync, shows a live equal-split preview, and wires the
// date quick-chips.
(function () {
  "use strict";
  var form = document.getElementById("expense-form");
  if (!form) return;

  // Switching the target group/direct reloads the form so the participant list
  // matches the new context (group members vs. friends).
  var retarget = form.querySelector("select[data-retarget]");
  if (retarget) {
    retarget.addEventListener("change", function () {
      window.location.href = retarget.value
        ? "/expenses/new?group=" + encodeURIComponent(retarget.value)
        : "/expenses/new";
    });
  }

  // Editing: changing the target group reloads the edit page for that group so
  // the participant list, the move warning, and the ack checkbox match it. The
  // server re-renders and enforces the ack on a real group change.
  var moveTarget = form.querySelector("select[data-move-target]");
  if (moveTarget) {
    moveTarget.addEventListener("change", function () {
      window.location.href = window.location.pathname +
        "?target=" + encodeURIComponent(moveTarget.value);
    });
  }

  var ZERO_DECIMAL = { JPY: 1, KRW: 1, VND: 1, CLP: 1, ISK: 1 };
  function decimals() {
    var c = form.querySelector('input[name="currency"]:checked');
    return (c && ZERO_DECIMAL[c.value]) ? 0 : 2;
  }
  function symbol() {
    var c = form.querySelector('input[name="currency"]:checked');
    return c ? c.value : "";
  }
  function amountMinor() {
    var el = form.querySelector('input[name="amount"]');
    var v = el ? parseFloat(el.value.replace(/[^0-9.]/g, "")) : NaN;
    if (!isFinite(v) || v <= 0) return 0;
    return Math.round(v * Math.pow(10, decimals()));
  }
  function fmt(minor) {
    var d = decimals();
    return (minor / Math.pow(10, d)).toFixed(d);
  }

  function currentMethod() {
    var m = form.querySelector('input[name="method"]:checked');
    return m ? m.value : "EQUAL";
  }

  // Live equal-split preview: floor + remainder, matching internal/split.
  function renderPreview() {
    var equal = currentMethod() === "EQUAL";
    var rows = form.querySelectorAll(".p-row");
    var included = [];
    rows.forEach(function (row) {
      if (row.querySelector("input.inc").checked) included.push(row);
    });
    var total = amountMinor(), n = included.length;
    var base = n ? Math.floor(total / n) : 0;
    var rem = n ? total - base * n : 0;
    rows.forEach(function (row) {
      var prev = row.querySelector(".p-preview");
      if (!prev) return;
      var idx = included.indexOf(row);
      if (equal && idx >= 0 && total > 0) {
        prev.textContent = symbol() + " " + fmt(base + (idx < rem ? 1 : 0));
      } else {
        prev.textContent = "";
      }
    });
  }

  // Reflect a chosen radio in its picker summary, then close the popover.
  function wirePicker(kind, render) {
    var picker = form.querySelector('[data-picker="' + kind + '"]');
    if (!picker) return;
    picker.addEventListener("change", function () {
      render(picker);
      picker.open = false;
    });
  }
  wirePicker("currency", function () {
    var btn = form.querySelector(".cur-btn");
    if (btn) btn.firstChild ? (btn.childNodes[0].nodeValue = symbol()) : (btn.textContent = symbol());
    renderPreview();
  });
  wirePicker("category", function (picker) {
    var opt = picker.querySelector("input:checked");
    if (!opt) return;
    var label = opt.closest(".picker-opt");
    var emo = label.querySelector("em"), sumEmo = picker.parentNode.querySelector(".cat-emo"),
        sumLbl = picker.parentNode.querySelector(".cat-lbl");
    if (sumEmo && emo) sumEmo.textContent = emo.textContent;
    if (sumLbl) sumLbl.textContent = opt.value;
  });
  wirePicker("paidby", function (picker) {
    var opt = picker.querySelector("input:checked");
    if (!opt) return;
    var src = opt.closest(".picker-opt");
    var sum = picker.querySelector("summary");
    var ava = src.querySelector(".ava").cloneNode(true);
    sum.textContent = "";
    sum.appendChild(ava);
    sum.appendChild(document.createTextNode(" " + src.textContent.trim()));
  });

  // Close any open picker when clicking outside it.
  document.addEventListener("click", function (e) {
    form.querySelectorAll(".picker[open]").forEach(function (p) {
      if (!p.contains(e.target)) p.open = false;
    });
  });

  // Date quick-chips.
  form.querySelectorAll(".date-chips .chip").forEach(function (chip) {
    chip.addEventListener("click", function () {
      var d = new Date();
      d.setDate(d.getDate() + parseInt(chip.getAttribute("data-days"), 10));
      var input = form.querySelector('input[name="date"]');
      if (input) input.value = d.toISOString().slice(0, 10);
      form.querySelectorAll(".date-chips .chip").forEach(function (c) { c.setAttribute("aria-pressed", c === chip); });
    });
  });

  form.addEventListener("input", renderPreview);
  form.addEventListener("change", renderPreview);
  renderPreview();
})();
