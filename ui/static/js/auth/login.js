const loginForm = document.getElementById('login-form');
const loginMessage = document.getElementById('login-message');
const verify2FAForm = document.getElementById('verify-2fa-form');

function storeAccessToken(token) {
  if (window.SmartRunApp?.setToken) {
    window.SmartRunApp.setToken(token);
    return;
  }
  localStorage.setItem('access_token', token);
}

if (loginForm && loginMessage) {
  if (verify2FAForm) {
    verify2FAForm.style.display = 'none';
  }

  loginForm.addEventListener('submit', async (event) => {
    event.preventDefault();
    loginMessage.textContent = 'Выполняем вход...';
    loginMessage.classList.remove('error');

    const formData = new FormData(loginForm);
    const payload = {
      email: String(formData.get('email') || '').trim(),
      password: String(formData.get('password') || '')
    };

    try {
      const response = await fetch('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(payload)
      });

      const text = await response.text();
      let data = null;

      try {
        data = text ? JSON.parse(text) : null;
      } catch (_) {}

      if (!response.ok) {
        const message = data?.message || text || 'Ошибка входа';
        loginMessage.textContent = message;
        loginMessage.classList.add('error');
        return;
      }

      if (data?.require_2fa) {
        loginMessage.textContent = 'Требуется 2FA: введите код и подтвердите.';
        if (verify2FAForm) {
          verify2FAForm.style.display = 'flex';
        }
        return;
      }

      if (data?.access_token) {
        storeAccessToken(data.access_token);
      }

      loginMessage.textContent = data?.message === 'ok' ? 'Успешный вход. Переходим в dashboard...' : 'Вход выполнен.';
      setTimeout(() => {
        window.location.href = '/dashboard';
      }, 400);
    } catch (error) {
      loginMessage.textContent = 'Сеть недоступна. Попробуйте еще раз.';
      loginMessage.classList.add('error');
    }
  });

  verify2FAForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
    loginMessage.textContent = 'Проверяем 2FA...';
    loginMessage.classList.remove('error');

    const formData = new FormData(verify2FAForm);
    const payload = {
      code: String(formData.get('code') || '').trim()
    };

    try {
      const response = await fetch('/api/verify-2fa', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(payload)
      });

      const text = await response.text();
      let data = null;
      try {
        data = text ? JSON.parse(text) : null;
      } catch (_) {}

      if (!response.ok) {
        const message = data?.message || text || 'Ошибка подтверждения 2FA';
        loginMessage.textContent = message;
        loginMessage.classList.add('error');
        return;
      }

      if (data?.access_token) {
        storeAccessToken(data.access_token);
      }

      loginMessage.textContent = '2FA подтверждена. Переходим в dashboard...';
      setTimeout(() => {
        window.location.href = '/dashboard';
      }, 350);
    } catch (error) {
      loginMessage.textContent = 'Сеть недоступна. Попробуйте еще раз.';
      loginMessage.classList.add('error');
    }
  });
}