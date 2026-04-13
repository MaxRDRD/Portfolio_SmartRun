function formatPace(value) {
  if (value == null || !Number.isFinite(Number(value))) {
    return '-';
  }
  const minutes = Math.floor(Number(value));
  const seconds = Math.round((Number(value) - minutes) * 60);
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')} /км`;
}

function formatDuration(seconds) {
  const total = Number(seconds || 0);
  if (!Number.isFinite(total) || total <= 0) {
    return '0 мин';
  }

  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);

  if (h > 0) {
    return `${h}ч ${String(m).padStart(2, '0')}м`;
  }
  return `${m} мин`;
}

function currentDateISO() {
  return new Date().toISOString().slice(0, 10);
}

function daysAgoISO(days) {
  const date = new Date();
  date.setDate(date.getDate() - days);
  return date.toISOString().slice(0, 10);
}

function safeNumber(value, fallback = 0) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

function onboardingKey(userId) {
  return `dashboard_onboarding_seen_${userId}`;
}

function markOnboardingSeen(userId) {
  if (!userId) {
    return;
  }
  localStorage.setItem(onboardingKey(userId), '1');
}

function hasSeenOnboarding(userId) {
  if (!userId) {
    return true;
  }
  return localStorage.getItem(onboardingKey(userId)) === '1';
}

function bindOnboarding(user) {
  const overlay = document.getElementById('dashboard-onboarding');
  const skipBtn = document.getElementById('onboarding-skip');
  const startBtn = document.getElementById('onboarding-start');
  if (!overlay || !user?.id || hasSeenOnboarding(user.id)) {
    return;
  }

  const close = () => {
    overlay.classList.add('hidden');
    overlay.setAttribute('aria-hidden', 'true');
    markOnboardingSeen(user.id);
  };

  overlay.classList.remove('hidden');
  overlay.setAttribute('aria-hidden', 'false');

  skipBtn?.addEventListener('click', close, { once: true });
  startBtn?.addEventListener('click', close, { once: true });
  overlay.addEventListener('click', (event) => {
    if (event.target === overlay) {
      close();
    }
  }, { once: true });
}

function estimateReadinessFromWorkouts(workouts) {
  if (!Array.isArray(workouts) || workouts.length === 0) {
    return null;
  }

  const recent = workouts.slice(0, 7);
  let total = 0;
  let count = 0;

  for (const w of recent) {
    const load = Number(w?.training_load);
    if (Number.isFinite(load) && load > 0) {
      total += load;
      count += 1;
      continue;
    }

    const durationHours = Number(w?.duration) / 3600;
    const intensity = Number(w?.intensity_factor);
    if (Number.isFinite(durationHours) && durationHours > 0 && Number.isFinite(intensity) && intensity > 0) {
      total += durationHours * intensity * intensity * 100;
      count += 1;
    }
  }

  if (count === 0) {
    return 75;
  }

  const avgLoad = total / count;
  const score = 95 - avgLoad*0.22;
  return Math.round(clamp(score, 25, 98));
}

function setText(id, value) {
  const element = document.getElementById(id);
  if (!element) {
    return;
  }
  element.textContent = value;
}

async function loadWindowMetrics(app, from, to) {
  return app.api(`/api/metrics?from=${from}&to=${to}`, { method: 'GET' }, true);
}

function applyWeeklyMetrics(metrics) {
  setText('stat-week-workouts', String(safeNumber(metrics?.total_workouts, 0)));
  setText('stat-week-distance', safeNumber(metrics?.total_distance, 0).toFixed(1));
}

function applyMonthlyMetrics(metrics) {
  setText('stat-month-workouts', String(safeNumber(metrics?.total_workouts, 0)));
  setText('stat-month-distance', safeNumber(metrics?.total_distance, 0).toFixed(1));
  setText('stat-month-pace', formatPace(metrics?.avg_pace));
  setText('stat-month-calories', String(Math.round(safeNumber(metrics?.total_calories, 0))));
}

async function loadDashboard() {
  const app = window.SmartRunApp;
  const user = app.user;

  if (!user) {
    return;
  }

  const title = document.getElementById('dashboard-title');
  if (title) {
    title.textContent = `${user.name || 'Ваш'} dashboard`;
  }

  bindOnboarding(user);

  setText('stat-week-workouts', '0');
  setText('stat-week-distance', '0.0');
  setText('stat-month-workouts', '0');
  setText('stat-month-distance', '0.0');
  setText('stat-month-pace', '-');
  setText('stat-month-calories', '0');
  setText('stat-readiness', '-');
  setText('stat-recommendation', '-');

  let workoutsForFallback = [];

  const to = currentDateISO();
  const weekFrom = daysAgoISO(6);
  const monthFrom = daysAgoISO(29);

  try {
    const [weekly, monthly] = await Promise.all([
      loadWindowMetrics(app, weekFrom, to),
      loadWindowMetrics(app, monthFrom, to)
    ]);

    applyWeeklyMetrics(weekly);
    applyMonthlyMetrics(monthly);
  } catch (_) {
    // keep defaults when metrics are unavailable
  }

  try {
    const daily = await app.api('/api/daily-metrics', { method: 'GET' }, true);
    if (Array.isArray(daily) && daily.length > 0) {
      const sorted = [...daily].sort((a, b) => String(b.date).localeCompare(String(a.date)));
      const latest = sorted[0];
      const latestPositive = sorted.find((x) => Number(x?.readiness_score) > 0) || latest;
      if (latestPositive?.readiness_score != null) {
        setText('stat-readiness', `${latestPositive.readiness_score}`);
      }
      if (latest?.recommendation) {
        setText('stat-recommendation', latest.recommendation);
      }
    }
  } catch (_) {
    // keep fallback placeholder
  }

  try {
    const workouts = await app.api('/api/workouts?limit=20&offset=0&sort_by=date&sort_order=desc', { method: 'GET' }, true);
    workoutsForFallback = Array.isArray(workouts) ? workouts : [];
  } catch (_) {
    workoutsForFallback = [];
  }

  const currentReadiness = Number(document.getElementById('stat-readiness')?.textContent);
  if (!Number.isFinite(currentReadiness) || currentReadiness <= 0) {
    const estimated = estimateReadinessFromWorkouts(workoutsForFallback);
    if (estimated != null) {
      setText('stat-readiness', String(estimated));
    }
  }
}

document.addEventListener('smartrun:ready', () => {
  loadDashboard();
});
