const toggle = document.getElementById('theme-toggle');

if (toggle) {
  const body = document.body;
  const root = document.documentElement;

  const applyTheme = (isDark) => {
    body.classList.toggle('dark', isDark);
    root.classList.toggle('dark', isDark);
    root.style.colorScheme = isDark ? 'dark' : 'light';
    toggle.textContent = isDark ? '☀️ Светлая' : '🌙 Темная';
  };

  if (
    localStorage.theme === 'dark' ||
    (!('theme' in localStorage) && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    applyTheme(true);
  }

  toggle.addEventListener('click', () => {
    const isDark = !body.classList.contains('dark');
    applyTheme(isDark);

    if (isDark) {
      localStorage.theme = 'dark';
      return;
    }

    localStorage.theme = 'light';
  });
}