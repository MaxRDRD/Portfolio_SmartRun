let profileMessageTimer = null;

function parseOptionalNumber(value) {
  if (value === '' || value == null) {
    return null;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function toPaceText(value) {
  if (value == null || !Number.isFinite(Number(value))) {
    return '';
  }
  const totalSeconds = Math.round(Number(value) * 60);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
}

function paceTextToMinutes(text) {
  const normalized = normalizePaceInput(String(text || '').trim());
  const match = /^([0-9]{1,2}):([0-5][0-9])$/.exec(normalized);
  if (!match) {
    return null;
  }
  const minutes = Number(match[1]);
  const seconds = Number(match[2]);
  return minutes + seconds / 60;
}

function normalizePaceInput(raw) {
  const digits = String(raw || '').replace(/\D/g, '').slice(0, 4);
  if (digits.length <= 2) {
    return digits;
  }

  const minPart = digits.slice(0, digits.length - 2);
  const secPart = digits.slice(-2);
  const sec = Math.min(59, Number(secPart));
  return `${minPart}:${String(sec).padStart(2, '0')}`;
}

function bindThresholdPaceAutoFormat() {
  const form = document.getElementById('profile-form');
  if (!form) {
    return;
  }

  const paceInput = form.elements.threshold_pace_text;
  if (!(paceInput instanceof HTMLInputElement)) {
    return;
  }

  paceInput.addEventListener('input', () => {
    const normalized = normalizePaceInput(paceInput.value);
    if (normalized !== paceInput.value) {
      paceInput.value = normalized;
    }
  });

  paceInput.addEventListener('blur', () => {
    const value = normalizePaceInput(paceInput.value);
    if (/^[0-9]{1,2}$/.test(value)) {
      paceInput.value = `${value}:00`;
      return;
    }
    paceInput.value = value;
  });
}

function setProfileMessage(message, isError = false) {
  const el = document.getElementById('profile-message');
  if (!el) {
    return;
  }
  el.textContent = message;
  el.classList.toggle('error', isError);

  if (profileMessageTimer) {
    window.clearTimeout(profileMessageTimer);
    profileMessageTimer = null;
  }

  if (message) {
    const ttl = isError ? 9000 : 4500;
    profileMessageTimer = window.setTimeout(() => {
      if (el.textContent === message) {
        el.textContent = '';
        el.classList.remove('error');
      }
      profileMessageTimer = null;
    }, ttl);
  }
}

function fillProfileForm(user) {
  const form = document.getElementById('profile-form');
  if (!form || !user) {
    return;
  }

  const fields = ['name', 'email', 'gender', 'age', 'weight_kg', 'height_cm', 'resting_hr', 'max_hr', 'weekly_runs'];
  fields.forEach((field) => {
    const input = form.elements[field];
    if (input && user[field] != null) {
      input.value = String(user[field]);
    }
  });

  const paceInput = form.elements.threshold_pace_text;
  if (paceInput) {
    paceInput.value = toPaceText(user.threshold_pace_min_km);
  }

  const requestEmail = document.querySelector('#password-request-form input[name="email"]');
  if (requestEmail && user.email) {
    requestEmail.value = user.email;
  }
}

function bindProfileForm() {
  const app = window.SmartRunApp;
  const form = document.getElementById('profile-form');

  form?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const f = new FormData(form);

    const payload = {};
    ['name', 'email', 'gender'].forEach((field) => {
      const value = String(f.get(field) || '').trim();
      if (value) {
        payload[field] = value;
      }
    });

    ['age', 'weight_kg', 'height_cm', 'resting_hr', 'max_hr', 'weekly_runs'].forEach((field) => {
      const parsed = parseOptionalNumber(f.get(field));
      if (parsed !== null) {
        payload[field] = parsed;
      }
    });

    const paceMinutes = paceTextToMinutes(f.get('threshold_pace_text'));
    if (String(f.get('threshold_pace_text') || '').trim() !== '' && paceMinutes == null) {
      setProfileMessage('Введите пороговый темп в формате мм:сс, например 04:30.', true);
      return;
    }
    if (paceMinutes != null) {
      payload.threshold_pace_min_km = paceMinutes;
    }

    try {
      const updated = await app.api('/api/me', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      }, true);
      app.user = updated;
      fillProfileForm(updated);
      setProfileMessage('Профиль обновлен.');
    } catch (error) {
      setProfileMessage(error.message, true);
    }
  });
}

function bindTOTP() {
  const app = window.SmartRunApp;
  const enableBtn = document.getElementById('enable-2fa-btn');
  const verifyForm = document.getElementById('verify-2fa-form');
  const block = document.getElementById('totp-block');
  const secretEl = document.getElementById('totp-secret');
  const linkEl = document.getElementById('totp-link');
  const qrEl = document.getElementById('totp-qr');

  enableBtn?.addEventListener('click', async () => {
    try {
      const data = await app.api('/api/enable-2fa', { method: 'POST' }, true);
      const secret = data?.secret || '';
      const userLabel = encodeURIComponent((app.user?.email || 'user').replace(/\s+/g, ''));
      const issuer = encodeURIComponent('SmartRun');
      const otpauth = `otpauth://totp/SmartRun:${userLabel}?secret=${encodeURIComponent(secret)}&issuer=${issuer}`;

      if (secretEl) {
        secretEl.textContent = secret;
      }
      if (linkEl) {
        linkEl.href = otpauth;
        linkEl.textContent = 'Открыть TOTP ссылку (desktop/mobile authenticator)';
      }
      if (qrEl) {
        qrEl.innerHTML = data?.qr_base64 ? `<img src="data:image/png;base64,${data.qr_base64}" alt="TOTP QR">` : '';
      }

      block?.classList.remove('hidden');
      setProfileMessage('Шаг 1/2: секрет создан. Сохраните его и подтвердите 6-значный код, чтобы включить 2FA.');
    } catch (error) {
      setProfileMessage(error.message, true);
    }
  });

  verifyForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const code = String(new FormData(verifyForm).get('code') || '').trim();

    try {
      const data = await app.api('/api/verify-2fa', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code })
      }, false);

      if (data?.access_token) {
        app.setToken(data.access_token);
      }
      setProfileMessage('2FA успешно включена и подтверждена.');
    } catch (error) {
      setProfileMessage(error.message, true);
    }
  });
}

function bindPasswordReset() {
  const app = window.SmartRunApp;
  const requestForm = document.getElementById('password-request-form');
  const confirmForm = document.getElementById('password-confirm-form');

  requestForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const email = String(new FormData(requestForm).get('email') || '').trim();

    try {
      await app.api('/api/password/reset/request', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email })
      }, false);
      setProfileMessage('Если email существует, ссылка для сброса уже отправлена.');
    } catch (error) {
      setProfileMessage(error.message, true);
    }
  });

  confirmForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const formData = new FormData(confirmForm);

    try {
      const data = await app.api('/api/password/reset/confirm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          token: String(formData.get('token') || '').trim(),
          new_password: String(formData.get('new_password') || '')
        })
      }, false);
      setProfileMessage(data?.message || 'Пароль обновлен.');
    } catch (error) {
      setProfileMessage(error.message, true);
    }
  });
}

document.addEventListener('smartrun:ready', () => {
  const app = window.SmartRunApp;
  if (!app.user) {
    return;
  }

  fillProfileForm(app.user);
  bindThresholdPaceAutoFormat();
  bindProfileForm();
  bindTOTP();
  bindPasswordReset();
});
