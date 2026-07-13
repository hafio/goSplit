/* GoSplit Web Push enrollment. Progressive enhancement: the button only does
 * anything when the browser supports push and the server has VAPID configured. */
(function () {
  function csrf() {
    var m = document.cookie.match(/(?:^|; )gosplit_csrf=([^;]+)/);
    return m ? decodeURIComponent(m[1]) : '';
  }
  function b64ToUint8(base64) {
    var pad = '='.repeat((4 - (base64.length % 4)) % 4);
    var b64 = (base64 + pad).replace(/-/g, '+').replace(/_/g, '/');
    var raw = atob(b64);
    var arr = new Uint8Array(raw.length);
    for (var i = 0; i < raw.length; i++) arr[i] = raw.charCodeAt(i);
    return arr;
  }

  async function enable(statusEl, btn) {
    var msg = btn.dataset;
    if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
      statusEl.textContent = msg.msgUnsupported || 'Push is not supported in this browser.';
      return;
    }
    var meta = await fetch('/push/public-key').then(function (r) { return r.json(); });
    if (!meta.enabled || !meta.publicKey) {
      statusEl.textContent = msg.msgUnconfigured || 'Push is not configured on this server.';
      return;
    }
    var perm = await Notification.requestPermission();
    if (perm !== 'granted') { statusEl.textContent = msg.msgDenied || 'Notifications permission denied.'; return; }

    var reg = await navigator.serviceWorker.ready;
    var sub = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: b64ToUint8(meta.publicKey),
    });
    await fetch('/push/subscribe', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf() },
      body: JSON.stringify(sub),
    });
    statusEl.textContent = msg.msgEnabled || 'Notifications enabled.';
  }

  async function test(statusEl, btn) {
    var msg = btn.dataset;
    var r = await fetch('/push/test', { method: 'POST', headers: { 'X-CSRF-Token': csrf() } });
    statusEl.textContent = r.ok
      ? (msg.msgTestSent || 'Test notification sent.')
      : (msg.msgTestFailed || 'Could not send test (is push configured?).');
  }

  document.addEventListener('DOMContentLoaded', function () {
    var enableBtn = document.getElementById('push-enable');
    var testBtn = document.getElementById('push-test');
    var status = document.getElementById('push-status');
    if (enableBtn) enableBtn.addEventListener('click', function () { enable(status, enableBtn); });
    if (testBtn) testBtn.addEventListener('click', function () { test(status, testBtn); });
  });
})();
