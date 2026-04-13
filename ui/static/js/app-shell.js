const SmartRunApp = {
  tokenKey: 'access_token',
  user: null,
  refreshPromise: null,
  dialogState: null,

  getToken() {
    return localStorage.getItem(this.tokenKey);
  },

  setToken(token) {
    if (!token) {
      return;
    }
    localStorage.setItem(this.tokenKey, token);
  },

  clearToken() {
    localStorage.removeItem(this.tokenKey);
  },

  async request(path, options = {}, withAuth = true) {
    const headers = Object.assign({}, options.headers || {});

    if (withAuth) {
      const token = this.getToken();
      if (token) {
        headers.Authorization = `Bearer ${token}`;
      }
    }

    const response = await fetch(path, {
      credentials: 'include',
      ...options,
      headers
    });

    const text = await response.text();
    let data = null;

    try {
      data = text ? JSON.parse(text) : null;
    } catch (_) {
      data = text;
    }

    return { response, data };
  },

  buildApiError(response, data) {
    const message = typeof data === 'string' ? data : data?.message || 'Request failed';
    return new Error(`${response.status}: ${message}`);
  },

  async refreshAccessToken() {
    if (this.refreshPromise) {
      return this.refreshPromise;
    }

    this.refreshPromise = (async () => {
      const { response, data } = await this.request('/api/refresh', { method: 'POST' }, false);
      if (!response.ok) {
        throw this.buildApiError(response, data);
      }

      const token = data?.access_token;
      if (!token) {
        throw new Error('Refresh succeeded without access token');
      }

      this.setToken(token);
      return token;
    })();

    try {
      return await this.refreshPromise;
    } finally {
      this.refreshPromise = null;
    }
  },

  async api(path, options = {}, withAuth = true) {
    let result = await this.request(path, options, withAuth);
    let refreshFailed = false;

    if (
      withAuth &&
      path !== '/api/refresh' &&
      result.response.status === 401
    ) {
      try {
        await this.refreshAccessToken();
        result = await this.request(path, options, true);
      } catch (_) {
        refreshFailed = true;
        this.clearToken();
      }
    }

    if (!result.response.ok) {
      if (withAuth && result.response.status === 401 && refreshFailed) {
        throw new Error('Сессия истекла. Выполните вход снова.');
      }
      throw this.buildApiError(result.response, result.data);
    }

    return result.data;
  },

  async restoreSession() {
    const token = this.getToken();

    try {
      if (!token) {
        await this.refreshAccessToken();
      }
      this.user = await this.api('/api/me', { method: 'GET' }, true);
      this.setHeaderAuthState(this.user);
      this.dispatchReady();
    } catch (_) {
      this.clearToken();
      this.user = null;
      this.setHeaderAuthState(null);
      this.dispatchReady();
    }
  },

  setHeaderAuthState(user) {
    const guestActions = document.querySelector('[data-guest-actions]');
    const userActions = document.querySelector('[data-user-actions]');
    const headerName = document.getElementById('header-user-name');

    if (guestActions) {
      guestActions.classList.toggle('hidden', Boolean(user));
    }

    if (userActions) {
      userActions.classList.toggle('hidden', !user);
    }

    if (headerName && user) {
      headerName.textContent = user.name || user.email || 'Профиль';
    }
  },

  dispatchReady() {
    document.dispatchEvent(new CustomEvent('smartrun:ready', { detail: { user: this.user } }));
  },

  ensureDialog() {
    if (this.dialogState) {
      return this.dialogState;
    }

    const root = document.createElement('div');
    root.className = 'app-dialog-overlay hidden';
    root.innerHTML = `
      <div class="app-dialog" role="dialog" aria-modal="true" aria-labelledby="app-dialog-title">
        <h3 id="app-dialog-title" class="app-dialog-title"></h3>
        <p class="app-dialog-message"></p>
        <div class="app-dialog-actions">
          <button type="button" class="btn btn-outline stable-btn app-dialog-cancel">Отмена</button>
          <button type="button" class="btn btn-primary stable-btn app-dialog-confirm">Подтвердить</button>
        </div>
      </div>
    `;

    document.body.appendChild(root);

    const state = {
      root,
      title: root.querySelector('.app-dialog-title'),
      message: root.querySelector('.app-dialog-message'),
      cancel: root.querySelector('.app-dialog-cancel'),
      confirm: root.querySelector('.app-dialog-confirm'),
      resolver: null,
      isAlert: false
    };

    const close = (value) => {
      if (state.resolver) {
        state.resolver(value);
        state.resolver = null;
      }
      root.classList.add('hidden');
    };

    state.cancel?.addEventListener('click', () => close(false));
    state.confirm?.addEventListener('click', () => close(true));

    root.addEventListener('click', (event) => {
      if (event.target === root) {
        close(false);
      }
    });

    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape' && !root.classList.contains('hidden')) {
        close(false);
      }
    });

    this.dialogState = state;
    return state;
  },

  confirm(message, options = {}) {
    const state = this.ensureDialog();
    state.isAlert = false;
    state.title.textContent = options.title || 'Подтверждение';
    state.message.textContent = message || '';
    state.cancel.textContent = options.cancelText || 'Отмена';
    state.confirm.textContent = options.confirmText || 'Подтвердить';
    state.cancel.classList.remove('hidden');
    state.root.classList.remove('hidden');

    return new Promise((resolve) => {
      state.resolver = resolve;
    });
  },

  alert(message, options = {}) {
    const state = this.ensureDialog();
    state.isAlert = true;
    state.title.textContent = options.title || 'Сообщение';
    state.message.textContent = message || '';
    state.confirm.textContent = options.confirmText || 'ОК';
    state.cancel.classList.add('hidden');
    state.root.classList.remove('hidden');

    return new Promise((resolve) => {
      state.resolver = () => resolve();
    });
  }
};

window.SmartRunApp = SmartRunApp;

document.addEventListener('DOMContentLoaded', async () => {
  const isProtected = document.body.dataset.protected === 'true';
  const isAuthPage = document.body.dataset.authPage === 'true';

  const logoutBtn = document.getElementById('logout-btn');
  logoutBtn?.addEventListener('click', async () => {
    try {
      await SmartRunApp.api('/api/logout', { method: 'POST' }, false);
    } catch (_) {
      // ignore logout API errors and clear local session anyway
    }
    SmartRunApp.clearToken();
    window.location.href = '/login';
  });

  await SmartRunApp.restoreSession();

  if (isProtected && !SmartRunApp.user) {
    window.location.href = '/login';
    return;
  }

  if (isAuthPage && SmartRunApp.user) {
    window.location.href = '/dashboard';
  }
});
