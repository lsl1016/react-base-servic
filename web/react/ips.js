(function () {
  'use strict';

  const AUTH_URL = '/react-base-service/react/playground/auth';
  const SDK_URL = '/react-base-service/react/sdk/0.0.1.js';
  const APP_URL = document.documentElement.dataset.appUrl || '/react-base-service/react/index.js';

  const authState = document.getElementById('auth-state');

  const showState = (message, isError) => {
    authState.textContent = message;
    authState.classList.toggle('rp-auth-error', isError === true);
  };

  const checkLogin = async () => {
    const response = await window.fetch(AUTH_URL, {
      method: 'GET',
      credentials: 'include',
      cache: 'no-store',
      headers: { Accept: 'application/json' },
    });

    const contentType = response.headers.get('content-type') || '';
    const payload = contentType.includes('application/json') ? await response.json() : null;
    const responseText = payload ? '' : await response.text();
    const code = Number(payload?.errNo ?? payload?.errno ?? payload?.code ?? 0);

    if (!response.ok || code !== 0) {
      throw new Error(payload?.errMsg || payload?.errmsg || responseText || '页面访问校验失败');
    }

    return true;
  };

  const loadModule = (src) => new Promise((resolve, reject) => {
    const script = document.createElement('script');
    script.type = 'module';
    script.src = src;
    script.onload = resolve;
    script.onerror = () => reject(new Error(`加载失败: ${src}`));
    document.body.appendChild(script);
  });

  const bootstrap = async () => {
    await checkLogin();
    await loadModule(SDK_URL);
    await loadModule(APP_URL);
    document.documentElement.classList.remove('rp-auth-pending');
    document.documentElement.classList.add('rp-auth-ready');
  };

  bootstrap().catch((error) => {
    showState(error?.message || '页面初始化失败', true);
  });
}());
