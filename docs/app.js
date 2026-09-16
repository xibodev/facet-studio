(function() {
  'use strict';

  // Theme Management (Default Light)
  const themeBtn = document.getElementById('theme-toggle-btn');
  const storedTheme = localStorage.getItem('facet-theme');
  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  
  const initialTheme = storedTheme || (prefersDark ? 'dark' : 'light');
  setTheme(initialTheme);

  if (themeBtn) {
    themeBtn.addEventListener('click', function() {
      const current = document.documentElement.getAttribute('data-theme') || 'light';
      const next = current === 'dark' ? 'light' : 'dark';
      setTheme(next);
      localStorage.setItem('facet-theme', next);
    });
  }

  function setTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    if (themeBtn) {
      themeBtn.textContent = theme === 'dark' ? 'LIGHT' : 'DARK';
      themeBtn.setAttribute('aria-label', theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme');
    }
  }

  // Install Command Switcher
  const installCommands = {
    win: 'irm https://xibodev.github.io/facet-studio/install.ps1 | iex',
    nix: 'curl -fsSL https://xibodev.github.io/facet-studio/install.sh | bash'
  };

  let activeOS = 'win';
  window.switchTab = function(os) {
    activeOS = os;
    document.querySelectorAll('.tab-btn').forEach(function(btn) {
      const isTarget = btn.getAttribute('data-os') === os;
      btn.classList.toggle('active', isTarget);
    });
    const codeEl = document.getElementById('install-code');
    if (codeEl) {
      codeEl.textContent = installCommands[os];
    }
  };

  // Copy helper
  window.copyInstallCmd = function() {
    const cmd = installCommands[activeOS];
    navigator.clipboard.writeText(cmd).then(function() {
      const btn = document.getElementById('copy-install-btn');
      if (btn) {
        btn.textContent = 'COPIED';
        setTimeout(function() {
          btn.textContent = 'COPY';
        }, 2000);
      }
    });
  };
})();
