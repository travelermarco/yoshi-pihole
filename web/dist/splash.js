/**
 * Yoshi Pi-hole — splash screen + page reveal.
 * Adattato da travelermarco/marco-style (js/splash.js).
 */
(function () {
  const SESSION_KEY = 'yoshi-pihole-splash';

  const splash = document.getElementById('splash');
  if (!splash) return;

  function reveal() {
    document.body.classList.add('revealed');
  }

  if (sessionStorage.getItem(SESSION_KEY)) {
    splash.classList.add('splash-gone');
    reveal();
    return;
  }

  sessionStorage.setItem(SESSION_KEY, '1');

  const fill = splash.querySelector('.splash-logo-fill');
  fill.addEventListener('animationend', () => {
    splash.classList.add('splash-out');
    reveal();
    splash.addEventListener('transitionend', () => splash.classList.add('splash-gone'), { once: true });
  }, { once: true });
})();
