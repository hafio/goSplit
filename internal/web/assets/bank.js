/* GoSplit Plaid Link flow. Progressive enhancement: only wires up when the
 * Plaid Link script loaded and the server exposes a link token. */
(function () {
  function csrf() {
    var m = document.cookie.match(/(?:^|; )gosplit_csrf=([^;]+)/);
    return m ? decodeURIComponent(m[1]) : '';
  }

  async function connect(statusEl, btn) {
    var msg = btn.dataset;
    if (typeof Plaid === 'undefined') { statusEl.textContent = msg.msgLinkFailed || 'Plaid Link failed to load.'; return; }
    var res = await fetch('/bank/link-token', { method: 'POST', headers: { 'X-CSRF-Token': csrf() } });
    if (!res.ok) { statusEl.textContent = msg.msgTokenFailed || 'Could not create a link token.'; return; }
    var linkToken = (await res.json()).linkToken;

    var handler = Plaid.create({
      token: linkToken,
      onSuccess: async function (publicToken) {
        var r = await fetch('/bank/exchange', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf() },
          body: JSON.stringify({ public_token: publicToken }),
        });
        statusEl.textContent = r.ok
          ? (msg.msgConnected || 'Bank connected. Click “Sync transactions”.')
          : (msg.msgConnectFailed || 'Could not connect the bank.');
      },
      onExit: function () {},
    });
    handler.open();
  }

  document.addEventListener('DOMContentLoaded', function () {
    var btn = document.getElementById('bank-connect');
    var status = document.getElementById('bank-status');
    if (btn) btn.addEventListener('click', function () { connect(status, btn); });
  });
})();
