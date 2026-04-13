let selectedWorkoutId = null;
let workoutMessageTimer = null;
let historyOffset = 0;
let historyLimit = 3;
let historyHasMore = true;
let historyLoading = false;
let historyMonths = [];
let expandedMonthKeys = new Set();
let filterMode = false;
let workoutsStateKey = '';
let historyObserver = null;
let activeWorkoutQuery = {
  limit: '50',
  offset: '0',
  sort_by: 'date',
  sort_order: 'desc'
};

function buildWorkoutsStateKey(user) {
  const identity = user?.id || user?.email || 'anon';
  return `smartrun:workouts:v1:${identity}`;
}

function saveWorkoutsScreenState() {
  if (!workoutsStateKey) {
    return;
  }

  const payload = {
    filterMode,
    activeWorkoutQuery,
    expandedMonthKeys: Array.from(expandedMonthKeys)
  };

  localStorage.setItem(workoutsStateKey, JSON.stringify(payload));
}

function restoreWorkoutsScreenState(user) {
  workoutsStateKey = buildWorkoutsStateKey(user);
  const raw = localStorage.getItem(workoutsStateKey);
  if (!raw) {
    return;
  }

  try {
    const parsed = JSON.parse(raw);
    if (parsed?.activeWorkoutQuery && typeof parsed.activeWorkoutQuery === 'object') {
      activeWorkoutQuery = {
        limit: '50',
        offset: '0',
        sort_by: 'date',
        sort_order: 'desc',
        ...parsed.activeWorkoutQuery
      };
    }
    if (Array.isArray(parsed?.expandedMonthKeys)) {
      expandedMonthKeys = new Set(parsed.expandedMonthKeys.map((v) => String(v)));
    }
    filterMode = Boolean(parsed?.filterMode);
  } catch (_) {
    // ignore invalid local state and fallback to defaults
  }
}

function parseOptionalNumber(value) {
  if (value === '' || value == null) {
    return null;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function parseHRZonesFromFormData(formData) {
  const hrZoneFields = [
    'zone_1_seconds',
    'zone_2_seconds',
    'zone_3_seconds',
    'zone_4_seconds',
    'zone_5_seconds',
    'zone_6_seconds'
  ];

  const hrRaw = hrZoneFields.map((name) => String(formData.get(name) ?? '').trim());
  const hasAnyZones = hrRaw.some((v) => v !== '');
  if (!hasAnyZones) {
    return { hrZones: null, error: null };
  }

  const requiredFilled = hrRaw.slice(0, 5).every((v) => v !== '');
  if (!requiredFilled) {
    return { hrZones: null, error: 'Для HR зон заполните минимум Z1-Z5 в формате HH:MM:SS.' };
  }

  const zoneSecs = hrRaw.map((v) => {
    if (v === '') {
      return 0;
    }
    return parseClockToSeconds(v, true);
  });
  const invalid = zoneSecs.some((n) => n == null || !Number.isFinite(n) || n < 0);
  if (invalid) {
    return { hrZones: null, error: 'HR зоны должны быть в формате HH:MM:SS, например 00:12:30.' };
  }

  return {
    hrZones: {
      zone_1_seconds: Math.round(zoneSecs[0]),
      zone_2_seconds: Math.round(zoneSecs[1]),
      zone_3_seconds: Math.round(zoneSecs[2]),
      zone_4_seconds: Math.round(zoneSecs[3]),
      zone_5_seconds: Math.round(zoneSecs[4]),
      zone_6_seconds: Math.round(zoneSecs[5])
    },
    error: null
  };
}

function formatPace(value) {
  if (value == null || !Number.isFinite(Number(value))) {
    return '-';
  }

  const totalSeconds = Math.round(Number(value) * 60);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')} /км`;
}

function formatDuration(seconds) {
  const total = Number(seconds || 0);
  if (!Number.isFinite(total) || total <= 0) {
    return '0 мин';
  }

  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;

  if (h > 0) {
    return `${h}ч ${String(m).padStart(2, '0')}м ${String(s).padStart(2, '0')}с`;
  }
  return `${m}м ${String(s).padStart(2, '0')}с`;
}

function formatDurationInput(seconds) {
  const total = Number(seconds || 0);
  if (!Number.isFinite(total) || total <= 0) {
    return '';
  }

  const hh = Math.floor(total / 3600);
  const mm = Math.floor((total % 3600) / 60);
  const ss = total % 60;
  return `${String(hh).padStart(2, '0')}:${String(mm).padStart(2, '0')}:${String(ss).padStart(2, '0')}`;
}

function formatDurationClockOrDash(seconds) {
  if (seconds == null || !Number.isFinite(Number(seconds)) || Number(seconds) < 0) {
    return '-';
  }

  const total = Math.floor(Number(seconds));
  const hh = Math.floor(total / 3600);
  const mm = Math.floor((total % 3600) / 60);
  const ss = total % 60;
  return `${String(hh).padStart(2, '0')}:${String(mm).padStart(2, '0')}:${String(ss).padStart(2, '0')}`;
}

function parseClockToSeconds(raw, allowZero = false) {
  const value = String(raw || '').trim();
  if (!value) {
    return null;
  }

  const parts = value.split(':').map((p) => p.trim());
  if (!parts.every((p) => /^[0-9]{1,2}$/.test(p))) {
    return null;
  }

  let hh = 0;
  let mm = 0;
  let ss = 0;

  if (parts.length === 3) {
    hh = Number(parts[0]);
    mm = Number(parts[1]);
    ss = Number(parts[2]);
  } else if (parts.length === 2) {
    mm = Number(parts[0]);
    ss = Number(parts[1]);
  } else if (parts.length === 1) {
    ss = Number(parts[0]);
  } else {
    return null;
  }

  if (mm > 59 || ss > 59) {
    return null;
  }

  const total = hh * 3600 + mm * 60 + ss;
  if (allowZero) {
    return total >= 0 ? total : null;
  }
  return total > 0 ? total : null;
}

function parseDurationToSeconds(raw) {
  return parseClockToSeconds(raw, false);
}

function normalizeDurationInput(raw) {
  const digits = String(raw || '').replace(/\D/g, '').slice(0, 6);
  if (!digits) {
    return '';
  }

  if (digits.length <= 2) {
    return digits;
  }

  if (digits.length <= 4) {
    const mm = digits.slice(0, digits.length - 2);
    const ss = digits.slice(-2);
    return `${mm}:${ss}`;
  }

  const hh = digits.slice(0, digits.length - 4);
  const mm = digits.slice(-4, -2);
  const ss = digits.slice(-2);
  return `${hh}:${mm}:${ss}`;
}

function bindDurationAutoFormat(formId, fieldName, allowZero = false) {
  const form = document.getElementById(formId);
  if (!form) {
    return;
  }

  const input = form.elements[fieldName];
  if (!(input instanceof HTMLInputElement)) {
    return;
  }

  input.addEventListener('input', () => {
    const normalized = normalizeDurationInput(input.value);
    if (normalized !== input.value) {
      input.value = normalized;
    }
  });

  input.addEventListener('blur', () => {
    const seconds = parseClockToSeconds(input.value, allowZero);
    if (seconds != null) {
      input.value = formatDurationInput(seconds);
    }
  });
}

function bindDurationAutoFormatMany(formId, fieldNames, allowZero = false) {
  fieldNames.forEach((fieldName) => bindDurationAutoFormat(formId, fieldName, allowZero));
}

function applyDateLimits() {
  const dateInput = document.querySelector('#workout-create-form input[name="date"]');
  if (!dateInput) {
    return;
  }

  dateInput.min = '1970-01-01';
  dateInput.max = new Date().toISOString().slice(0, 10);
}

function setWorkoutMessage(message, isError = false) {
  const el = document.getElementById('workouts-message');
  if (!el) {
    return;
  }
  el.textContent = message;
  el.classList.toggle('error', isError);

  if (workoutMessageTimer) {
    window.clearTimeout(workoutMessageTimer);
    workoutMessageTimer = null;
  }

  if (message) {
    const ttl = isError ? 9000 : 4500;
    workoutMessageTimer = window.setTimeout(() => {
      if (el.textContent === message) {
        el.textContent = '';
        el.classList.remove('error');
      }
      workoutMessageTimer = null;
    }, ttl);
  }
}

function escapeHtml(value) {
  return String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;');
}

function toDateInputValue(dateStr) {
  const str = String(dateStr || '');
  return str.length >= 10 ? str.slice(0, 10) : '';
}

function openDetailsPanel() {
  document.getElementById('workout-details-panel')?.classList.remove('hidden');
}

function closeDetailsPanel() {
  selectedWorkoutId = null;
  document.getElementById('workout-details-panel')?.classList.add('hidden');
}

function getWorkoutIdFromPath() {
  const match = /^\/workouts\/(\d+)$/.exec(window.location.pathname);
  if (!match) {
    return null;
  }
  const id = Number(match[1]);
  return Number.isFinite(id) && id > 0 ? id : null;
}

function isWorkoutDetailsRoute() {
  return getWorkoutIdFromPath() != null;
}

function applyDetailsRouteMode() {
  if (!isWorkoutDetailsRoute()) {
    return;
  }

  const createSection = document.getElementById('create-workout-panel')?.closest('section');
  const fitSection = document.getElementById('create-fit-panel')?.closest('section');
  const monthsSection = document.getElementById('workouts-months-panel');
  const filterOverlay = document.getElementById('workouts-filter-overlay');
  const detailsPanel = document.getElementById('workout-details-panel');

  createSection?.classList.add('hidden');
  fitSection?.classList.add('hidden');
  monthsSection?.classList.add('hidden');
  filterOverlay?.classList.add('hidden');
  detailsPanel?.classList.remove('hidden');
}

function applyWorkoutQueryToForm() {
  const form = document.getElementById('workouts-filter-form');
  if (!(form instanceof HTMLFormElement)) {
    return;
  }

  for (const [key, value] of Object.entries(activeWorkoutQuery)) {
    const field = form.elements.namedItem(key);
    if (field instanceof HTMLInputElement || field instanceof HTMLSelectElement) {
      field.value = value;
    }
  }
}

function buildWorkoutQueryFromForm(form) {
  const fd = new FormData(form);
  const query = { offset: '0' };

  const fields = [
    'type',
    'from',
    'to',
    'min_distance',
    'max_distance',
    'min_avg_hr',
    'max_avg_hr',
    'min_pace',
    'max_pace',
    'min_rpe',
    'max_rpe',
    'has_notes',
    'has_hr_zones',
    'shoes',
    'sort_by',
    'sort_order',
    'limit'
  ];

  fields.forEach((field) => {
    const value = String(fd.get(field) ?? '').trim();
    if (!value) {
      return;
    }
    query[field] = value;
  });

  if (!query.sort_by) {
    query.sort_by = 'date';
  }
  if (!query.sort_order) {
    query.sort_order = 'desc';
  }
  if (!query.limit) {
    query.limit = '50';
  }

  return query;
}

function buildWorkoutQueryString() {
  const params = new URLSearchParams();
  Object.entries(activeWorkoutQuery).forEach(([key, value]) => {
    const normalized = String(value ?? '').trim();
    if (normalized) {
      params.set(key, normalized);
    }
  });
  return params.toString();
}

function setInputValue(id, value) {
  const input = document.getElementById(id);
  if (!input) {
    return;
  }
  input.value = value == null ? '' : String(value);
}

function formatNumber(value, digits = 1) {
  const n = Number(value);
  if (!Number.isFinite(n)) {
    return '-';
  }
  return n.toFixed(digits);
}

function formatInt(value) {
  const n = Number(value);
  if (!Number.isFinite(n)) {
    return '-';
  }
  return String(Math.round(n));
}

function formatMinutes(value) {
  const n = Number(value);
  if (!Number.isFinite(n)) {
    return '-';
  }

  const total = Math.round(n);
  if (total >= 1440) {
    const days = Math.floor(total / 1440);
    const hours = Math.floor((total % 1440) / 60);
    return `${days}д ${hours}ч`;
  }
  if (total >= 60) {
    const hours = Math.floor(total / 60);
    const mins = total % 60;
    return `${hours}ч ${mins}м`;
  }
  return `${total} мин`;
}

function chip(title, value) {
  return `<div class="detail-chip"><div class="detail-chip-title">${escapeHtml(title)}</div><div class="detail-chip-value">${escapeHtml(value)}</div></div>`;
}

function section(title, chips) {
  return `<section class="detail-section"><h3 class="detail-section-title">${escapeHtml(title)}</h3><div class="details-grid-compact">${chips.join('')}</div></section>`;
}

function renderWorkoutDetails(workout) {
  const view = document.getElementById('workout-details-view');
  if (!view) {
    return;
  }

  const basic = section('Базовые', [
    chip('Тип', workout.type_activity || '-'),
    chip('Дата', workout.date || '-'),
    chip('Дистанция', `${formatNumber(workout.distance, 2)} км`),
    chip('Длительность', formatDuration(workout.duration)),
    chip('Темп', formatPace(workout.pace)),
    chip('Калории', formatInt(workout.calories))
  ]);

  const cardio = section('Кардио и техника', [
    chip('Пульс avg/max', `${formatInt(workout.avg_hr)} / ${formatInt(workout.max_hr)}`),
    chip('Каденс avg/max', `${formatInt(workout.avg_cadence)} / ${formatInt(workout.max_cadence)}`),
    chip('RPE', formatInt(workout.perceived_effort)),
    chip('Stress avg', formatInt(workout.avg_stress)),
    chip('Набор высоты', `${formatNumber(workout.elevation_gain, 1)} м`),
    chip('Сброс высоты', `${formatNumber(workout.elevation_loss, 1)} м`)
  ]);

  const load = section('Нагрузка и эффект', [
    chip('Training Load', formatNumber(workout.training_load, 1)),
    chip('TSS', formatNumber(workout.training_stress_score, 1)),
    chip('Intensity Factor', formatNumber(workout.intensity_factor, 2)),
    chip('Aerobic Effect', formatNumber(workout.aerobic_training_effect, 1)),
    chip('Anaerobic Effect', formatNumber(workout.anaerobic_training_effect, 1)),
    chip('Primary Focus', workout.primary_training_focus || '-')
  ]);

  const recovery = section('Восстановление и HRV', [
    chip('Recovery Time', formatMinutes(workout.recovery_time)),
    chip('VO2max estimate', formatNumber(workout.vo2max_estimate, 1)),
    chip('Efficiency', formatNumber(workout.efficiency, 3)),
    chip('SDRR HRV', formatInt(workout.sdrr_hrv)),
    chip('RMSSD HRV', formatInt(workout.rmssd_hrv)),
    chip('Кроссовки', workout.shoes || '-')
  ]);

  const zones = workout.hr_zones || {};
  const zonesSection = section('HR зоны', [
    chip('Z1-Z3 (HH:MM:SS)', `${formatDurationClockOrDash(zones.zone_1_seconds)} / ${formatDurationClockOrDash(zones.zone_2_seconds)} / ${formatDurationClockOrDash(zones.zone_3_seconds)}`),
    chip('Z4-Z6 (HH:MM:SS)', `${formatDurationClockOrDash(zones.zone_4_seconds)} / ${formatDurationClockOrDash(zones.zone_5_seconds)} / ${formatDurationClockOrDash(zones.zone_6_seconds)}`)
  ]);

  const notes = section('Комментарий', [
    chip('Заметки', workout.notes || '-')
  ]);

  view.innerHTML = `${basic}${cardio}${load}${recovery}${zonesSection}${notes}`;
}

function fillEditForm(workout) {
  setInputValue('workout-edit-id', workout.id);
  setInputValue('workout-edit-date', toDateInputValue(workout.date));
  setInputValue('workout-edit-distance', workout.distance ?? '');
  setInputValue('workout-edit-duration', formatDurationInput(workout.duration));
  setInputValue('workout-edit-type', workout.type_activity || 'run');
  setInputValue('workout-edit-avg-hr', workout.avg_hr ?? '');
  setInputValue('workout-edit-max-hr', workout.max_hr ?? '');
  setInputValue('workout-edit-calories', workout.calories ?? '');
  setInputValue('workout-edit-avg-cadence', workout.avg_cadence ?? '');
  setInputValue('workout-edit-max-cadence', workout.max_cadence ?? '');
  setInputValue('workout-edit-elevation-gain', workout.elevation_gain ?? '');
  setInputValue('workout-edit-elevation-loss', workout.elevation_loss ?? '');
  setInputValue('workout-edit-rpe', workout.perceived_effort ?? '');
  setInputValue('workout-edit-notes', workout.notes ?? '');
  setInputValue('workout-edit-shoes', workout.shoes ?? '');
  setInputValue('workout-edit-zone-1', formatDurationInput(workout.hr_zones?.zone_1_seconds));
  setInputValue('workout-edit-zone-2', formatDurationInput(workout.hr_zones?.zone_2_seconds));
  setInputValue('workout-edit-zone-3', formatDurationInput(workout.hr_zones?.zone_3_seconds));
  setInputValue('workout-edit-zone-4', formatDurationInput(workout.hr_zones?.zone_4_seconds));
  setInputValue('workout-edit-zone-5', formatDurationInput(workout.hr_zones?.zone_5_seconds));
  setInputValue('workout-edit-zone-6', formatDurationInput(workout.hr_zones?.zone_6_seconds));
}

async function openWorkoutDetails(id) {
  const app = window.SmartRunApp;
  const workoutId = Number(id);
  if (!Number.isFinite(workoutId) || workoutId <= 0) {
    setWorkoutMessage('Некорректный id тренировки.', true);
    return;
  }

  try {
    const workout = await app.api(`/api/workouts/${workoutId}`, { method: 'GET' }, true);
    selectedWorkoutId = workoutId;
    renderWorkoutDetails(workout);
    fillEditForm(workout);
    openDetailsPanel();
  } catch (error) {
    setWorkoutMessage(error.message, true);
  }
}

function renderWorkouts(items) {
  const list = document.getElementById('workouts-list');
  const empty = document.getElementById('workouts-empty');
  if (!list || !empty) {
    return;
  }

  if (!Array.isArray(items) || items.length === 0) {
    list.innerHTML = '';
    empty.classList.remove('hidden');
    return;
  }

  empty.classList.add('hidden');
  list.innerHTML = items
    .map((w) => {
      return `<article class="list-item card-row workout-open-card" role="button" tabindex="0" data-workout-id="${w.id}">
        <div class="workout-item-main">
          <h3>${w.type_activity || 'Тренировка'}</h3>
          <p>${w.date || '-'} • ${w.distance ?? 0} км • ${formatDuration(w.duration)}</p>
          <p class="muted-text">Пульс: ${w.avg_hr ?? '-'} / ${w.max_hr ?? '-'} • Калории: ${w.calories ?? '-'}</p>
        </div>
      </article>`;
    })
    .join('');
}

function formatMonthLabel(raw) {
  const value = String(raw || '').trim();
  if (!value) {
    return '-';
  }

  const normalized = value.length === 7 ? `${value}-01` : value;
  const dt = new Date(normalized);
  if (Number.isNaN(dt.getTime())) {
    return value;
  }
  return dt.toLocaleDateString('ru-RU', { month: 'long', year: 'numeric' });
}

function getMonthKey(raw) {
  return String(raw || '').trim().slice(0, 7);
}

function shouldMonthBeExpanded(monthKey, index) {
  if (expandedMonthKeys.has(monthKey)) {
    return true;
  }
  if (expandedMonthKeys.size === 0 && index === 0) {
    expandedMonthKeys.add(monthKey);
    return true;
  }
  return false;
}

function monthMetricChip(title, value) {
  return `<span class="month-metric-chip"><strong>${escapeHtml(title)}:</strong> ${escapeHtml(value)}</span>`;
}

function renderHistory(months) {
  const list = document.getElementById('workouts-history-list');
  const empty = document.getElementById('workouts-history-empty');
  const actions = document.getElementById('workouts-history-actions');
  const loadMoreBtn = document.getElementById('workouts-history-load-more');
  if (!list || !empty) {
    return;
  }

  if (!Array.isArray(months) || months.length === 0) {
    list.innerHTML = '';
    empty.classList.remove('hidden');
    actions?.classList.add('hidden');
    return;
  }

  empty.classList.add('hidden');
  actions?.classList.remove('hidden');
  if (loadMoreBtn) {
    loadMoreBtn.classList.toggle('hidden', !historyHasMore);
  }

  list.innerHTML = months
    .map((month, index) => {
      const monthKey = getMonthKey(month.month);
      const expanded = shouldMonthBeExpanded(monthKey, index);
      const workouts = Array.isArray(month.workouts) ? month.workouts : [];
      const avgPace = Number(month.total_distance) > 0
        ? Number(month.total_duration) / 60 / Number(month.total_distance)
        : null;

      const items = workouts
        .map((w) => {
          return `<article class="list-item month-workout-row workout-open-card" role="button" tabindex="0" data-workout-id="${w.id}">
            <div class="workout-item-main">
              <h3>${escapeHtml(w.type_activity || 'Тренировка')}</h3>
              <p>${escapeHtml(w.date || '-')} • ${formatNumber(w.distance, 2)} км • ${formatDuration(w.duration)}</p>
              <p class="muted-text">Темп: ${formatPace(w.pace)}</p>
            </div>
          </article>`;
        })
        .join('');

      return `<article class="list-item month-block">
        <div class="month-block-head">
          <button class="month-toggle" type="button" data-month-key="${escapeHtml(monthKey)}" aria-expanded="${expanded ? 'true' : 'false'}">
            <span class="month-title">${escapeHtml(formatMonthLabel(month.month))}</span>
            <span class="month-toggle-icon">${expanded ? '−' : '+'}</span>
          </button>
          <div class="month-metrics">
            ${monthMetricChip('Км', `${formatNumber(month.total_distance, 1)} км`)}
            ${monthMetricChip('Тренировок', formatInt(month.workouts_count))}
            ${monthMetricChip('Время', formatDuration(month.total_duration))}
            ${avgPace != null ? monthMetricChip('Средний темп', formatPace(avgPace)) : ''}
          </div>
        </div>
        <div class="month-block-body ${expanded ? '' : 'hidden'}" data-month-body="${escapeHtml(monthKey)}">
          ${items || '<p class="muted-text">Нет тренировок в месяце.</p>'}
        </div>
      </article>`;
    })
    .join('');
}

async function loadHistory(append = true) {
  const app = window.SmartRunApp;
  historyLoading = true;
  try {
    const query = new URLSearchParams({
      limit: String(historyLimit),
      offset: String(historyOffset)
    });

    const data = await app.api(`/api/workouts/history?${query.toString()}`, { method: 'GET' }, true);
    const batch = Array.isArray(data?.months) ? data.months : [];
    historyHasMore = batch.length >= historyLimit;
    historyMonths = append ? [...historyMonths, ...batch] : batch;
    renderHistory(historyMonths);
  } catch (error) {
    renderHistory([]);
    setWorkoutMessage(error.message, true);
  } finally {
    historyLoading = false;
  }
}

async function loadMoreHistory() {
  historyOffset += historyLimit;
  await loadHistory(true);
}

async function resetAndLoadHistory() {
  historyOffset = 0;
  historyMonths = [];
  historyHasMore = true;
  await loadHistory(false);
}

function bindHistoryInfiniteScroll() {
  const sentinel = document.getElementById('workouts-history-sentinel');
  if (!sentinel || typeof IntersectionObserver === 'undefined') {
    return;
  }

  if (historyObserver) {
    historyObserver.disconnect();
  }

  historyObserver = new IntersectionObserver(async (entries) => {
    const entry = entries[0];
    if (!entry?.isIntersecting) {
      return;
    }
    if (filterMode || historyLoading || !historyHasMore) {
      return;
    }
    await loadMoreHistory();
  }, {
    root: null,
    rootMargin: '180px 0px',
    threshold: 0.01
  });

  historyObserver.observe(sentinel);
}

function bindHistory() {
  const list = document.getElementById('workouts-history-list');
  const loadMoreBtn = document.getElementById('workouts-history-load-more');

  loadMoreBtn?.addEventListener('click', async () => {
    if (!historyHasMore) {
      return;
    }
    await loadMoreHistory();
  });

  list?.addEventListener('click', (event) => {
    const target = event.target;
    if (!(target instanceof HTMLElement)) {
      return;
    }

    const toggle = target.closest('.month-toggle');
    if (!(toggle instanceof HTMLElement)) {
      return;
    }

    const key = String(toggle.dataset.monthKey || '');
    const body = list.querySelector(`[data-month-body="${key}"]`);
    if (!(body instanceof HTMLElement)) {
      return;
    }

    const hidden = body.classList.toggle('hidden');
    toggle.setAttribute('aria-expanded', hidden ? 'false' : 'true');
    if (hidden) {
      expandedMonthKeys.delete(key);
    } else {
      expandedMonthKeys.add(key);
    }
    saveWorkoutsScreenState();
    const icon = toggle.querySelector('.month-toggle-icon');
    if (icon) {
      icon.textContent = hidden ? '+' : '−';
    }
  });
}

async function loadWorkouts() {
  const app = window.SmartRunApp;
  try {
    const queryString = buildWorkoutQueryString();
    const url = queryString ? `/api/workouts?${queryString}` : '/api/workouts';
    const workouts = await app.api(url, { method: 'GET' }, true);
    renderWorkouts(workouts);
  } catch (error) {
    renderWorkouts([]);
    setWorkoutMessage(error.message, true);
  }
}

function setFilterMode(enabled) {
  filterMode = enabled;

  const results = document.getElementById('workouts-filter-results');
  const historyList = document.getElementById('workouts-history-list');
  const historyActions = document.getElementById('workouts-history-actions');
  const historyEmpty = document.getElementById('workouts-history-empty');
  const exitBtn = document.getElementById('workouts-exit-filters');

  results?.classList.toggle('hidden', !enabled);
  historyList?.classList.toggle('hidden', enabled);
  historyActions?.classList.toggle('hidden', enabled);
  historyEmpty?.classList.toggle('hidden', enabled);
  exitBtn?.classList.toggle('hidden', !enabled);
  saveWorkoutsScreenState();
}

function closeCreatePanels() {
  document.getElementById('create-workout-panel')?.classList.add('hidden');
  document.getElementById('create-fit-panel')?.classList.add('hidden');
}

function updateQuickFiltersUI() {
  const notesBtn = document.getElementById('quick-filter-has-notes');
  const zonesBtn = document.getElementById('quick-filter-has-hr-zones');

  notesBtn?.classList.toggle('active', activeWorkoutQuery.has_notes === 'true');
  zonesBtn?.classList.toggle('active', activeWorkoutQuery.has_hr_zones === 'true');
}

async function applyQuickFilters() {
  setFilterMode(true);
  updateQuickFiltersUI();
  await loadWorkouts();
}

function bindQuickFilters() {
  const notesBtn = document.getElementById('quick-filter-has-notes');
  const zonesBtn = document.getElementById('quick-filter-has-hr-zones');
  const resetBtn = document.getElementById('quick-filter-reset');

  updateQuickFiltersUI();

  notesBtn?.addEventListener('click', async () => {
    if (activeWorkoutQuery.has_notes === 'true') {
      delete activeWorkoutQuery.has_notes;
    } else {
      activeWorkoutQuery.has_notes = 'true';
    }
    saveWorkoutsScreenState();
    await applyQuickFilters();
  });

  zonesBtn?.addEventListener('click', async () => {
    if (activeWorkoutQuery.has_hr_zones === 'true') {
      delete activeWorkoutQuery.has_hr_zones;
    } else {
      activeWorkoutQuery.has_hr_zones = 'true';
    }
    saveWorkoutsScreenState();
    await applyQuickFilters();
  });

  resetBtn?.addEventListener('click', async () => {
    delete activeWorkoutQuery.has_notes;
    delete activeWorkoutQuery.has_hr_zones;
    updateQuickFiltersUI();
    saveWorkoutsScreenState();
    setFilterMode(false);
    await resetAndLoadHistory();
  });
}

function openFilterDialog() {
  document.getElementById('workouts-filter-overlay')?.classList.remove('hidden');
}

function closeFilterDialog() {
  document.getElementById('workouts-filter-overlay')?.classList.add('hidden');
}

function bindFilters() {
  const form = document.getElementById('workouts-filter-form');
  const resetBtn = document.getElementById('workouts-filter-reset');
  const openBtn = document.getElementById('workouts-open-filters');
  const cancelBtn = document.getElementById('workouts-filter-cancel');
  const exitBtn = document.getElementById('workouts-exit-filters');
  const overlay = document.getElementById('workouts-filter-overlay');
  if (!(form instanceof HTMLFormElement)) {
    return;
  }

  openBtn?.addEventListener('click', () => openFilterDialog());
  cancelBtn?.addEventListener('click', () => closeFilterDialog());
  overlay?.addEventListener('click', (event) => {
    if (event.target === overlay) {
      closeFilterDialog();
    }
  });

  applyWorkoutQueryToForm();

  form.addEventListener('submit', async (event) => {
    event.preventDefault();
    activeWorkoutQuery = buildWorkoutQueryFromForm(form);
    saveWorkoutsScreenState();
    setFilterMode(true);
    updateQuickFiltersUI();
    closeFilterDialog();
    await loadWorkouts();
  });

  resetBtn?.addEventListener('click', async () => {
    activeWorkoutQuery = {
      limit: '50',
      offset: '0',
      sort_by: 'date',
      sort_order: 'desc'
    };
    form.reset();
    applyWorkoutQueryToForm();
    saveWorkoutsScreenState();
    setFilterMode(false);
    updateQuickFiltersUI();
    closeFilterDialog();
    await resetAndLoadHistory();
  });

  exitBtn?.addEventListener('click', async () => {
    setFilterMode(false);
    updateQuickFiltersUI();
    await resetAndLoadHistory();
  });
}

function bindCreatePanel() {
  const createToggle = document.getElementById('create-workout-toggle');
  const fitToggle = document.getElementById('create-fit-toggle');
  const createPanel = document.getElementById('create-workout-panel');
  const fitPanel = document.getElementById('create-fit-panel');

  createToggle?.addEventListener('click', () => {
    fitPanel?.classList.add('hidden');
    createPanel?.classList.toggle('hidden');
  });

  fitToggle?.addEventListener('click', () => {
    createPanel?.classList.add('hidden');
    fitPanel?.classList.toggle('hidden');
  });
}

function bindCreateForm() {
  const app = window.SmartRunApp;
  const form = document.getElementById('workout-create-form');

  form?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const f = new FormData(form);

    const durationSeconds = parseDurationToSeconds(f.get('duration'));
    if (durationSeconds == null) {
      setWorkoutMessage('Введите длительность в формате HH:MM:SS, например 01:05:30.', true);
      return;
    }

    const payload = {
      date: String(f.get('date') || ''),
      distance: Number(f.get('distance')),
      duration: durationSeconds,
      type_activity: String(f.get('type_activity') || '').trim() || 'run'
    };

    ['avg_hr', 'max_hr', 'calories', 'avg_cadence', 'rpe'].forEach((field) => {
      const parsed = parseOptionalNumber(f.get(field));
      if (parsed !== null) {
        payload[field] = parsed;
      }
    });

    const notes = String(f.get('notes') || '').trim();
    const shoes = String(f.get('shoes') || '').trim();
    if (notes) {
      payload.notes = notes;
    }
    if (shoes) {
      payload.shoes = shoes;
    }

    const zones = parseHRZonesFromFormData(f);
    if (zones.error) {
      setWorkoutMessage(zones.error, true);
      return;
    }
    if (zones.hrZones) {
      payload.hr_zones = zones.hrZones;
    }

    try {
      await app.api('/api/workouts', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      }, true);
      setWorkoutMessage('Тренировка добавлена.');
      form.reset();
      closeCreatePanels();
      setFilterMode(false);
      updateQuickFiltersUI();
      await resetAndLoadHistory();
    } catch (error) {
      setWorkoutMessage(error.message, true);
    }
  });
}

function bindFitImport() {
  const app = window.SmartRunApp;
  const form = document.getElementById('workout-fit-form');

  form?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const f = new FormData(form);
    const file = f.get('fit_file');

    if (!(file instanceof File) || file.size === 0) {
      setWorkoutMessage('Выберите FIT файл.', true);
      return;
    }

    try {
      const token = app.getToken();
      const headers = token ? { Authorization: `Bearer ${token}` } : {};
      headers['Content-Type'] = file.type || 'application/octet-stream';
      const response = await fetch('/api/workouts', {
        method: 'POST',
        headers,
        credentials: 'include',
        body: file
      });

      const text = await response.text();
      let data = text;
      try {
        data = text ? JSON.parse(text) : null;
      } catch (_) {}

      if (!response.ok) {
        const msg = typeof data === 'string' ? data : data?.message || 'Ошибка импорта FIT';
        throw new Error(msg);
      }

      const createdId = Number(data?.id);
      if (!Number.isFinite(createdId) || createdId <= 0) {
        throw new Error('Сервер вернул успех, но тренировка не была подтверждена (нет id).');
      }

      setWorkoutMessage('FIT файл импортирован.');
      form.reset();
      closeCreatePanels();
      setFilterMode(false);
      updateQuickFiltersUI();
      await resetAndLoadHistory();
    } catch (error) {
      setWorkoutMessage(error.message, true);
    }
  });
}

function bindListActions() {
  const openFromNode = (target) => {
    const card = target.closest('.workout-open-card');
    if (!(card instanceof HTMLElement)) {
      return;
    }
    const id = Number(card.dataset.workoutId);
    if (!Number.isFinite(id) || id <= 0) {
      return;
    }
    window.location.href = `/workouts/${id}`;
  };

  const list = document.getElementById('workouts-list');
  const history = document.getElementById('workouts-history-list');

  list?.addEventListener('click', (event) => {
    const target = event.target;
    if (!(target instanceof HTMLElement)) {
      return;
    }
    openFromNode(target);
  });

  history?.addEventListener('click', (event) => {
    const target = event.target;
    if (!(target instanceof HTMLElement)) {
      return;
    }
    if (target.closest('.month-toggle')) {
      return;
    }
    openFromNode(target);
  });

  const onKey = (event) => {
    if (event.key !== 'Enter' && event.key !== ' ') {
      return;
    }
    const target = event.target;
    if (!(target instanceof HTMLElement) || !target.classList.contains('workout-open-card')) {
      return;
    }
    event.preventDefault();
    openFromNode(target);
  };

  list?.addEventListener('keydown', onKey);
  history?.addEventListener('keydown', onKey);
}

function buildUpdatePayloadFromForm(form) {
  const f = new FormData(form);
  const durationSeconds = parseDurationToSeconds(f.get('duration'));
  if (durationSeconds == null) {
    return { error: 'Введите длительность в формате HH:MM:SS, например 01:05:30.' };
  }

  const payload = {
    date: String(f.get('date') || ''),
    distance: Number(f.get('distance')),
    duration: durationSeconds,
    type_activity: String(f.get('type_activity') || '').trim() || 'run',
    notes: String(f.get('notes') || '').trim(),
    shoes: String(f.get('shoes') || '').trim()
  };

  ['avg_hr', 'max_hr', 'calories', 'avg_cadence', 'max_cadence', 'rpe'].forEach((field) => {
    const raw = f.get(field);
    if (raw === '' || raw == null) {
      return;
    }
    const parsed = Number(raw);
    if (Number.isFinite(parsed)) {
      payload[field] = parsed;
    }
  });

  ['elevation_gain', 'elevation_loss'].forEach((field) => {
    const raw = f.get(field);
    if (raw === '' || raw == null) {
      return;
    }
    const parsed = Number(raw);
    if (Number.isFinite(parsed)) {
      payload[field] = parsed;
    }
  });

  const zones = parseHRZonesFromFormData(f);
  if (zones.error) {
    return { error: zones.error };
  }
  if (zones.hrZones) {
    payload.hr_zones = zones.hrZones;
  }

  return { payload };
}

function bindEditForm() {
  const app = window.SmartRunApp;
  const form = document.getElementById('workout-edit-form');
  form?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const id = Number(document.getElementById('workout-edit-id')?.value || selectedWorkoutId);
    if (!Number.isFinite(id) || id <= 0) {
      setWorkoutMessage('Сначала выберите тренировку.', true);
      return;
    }

    try {
      const result = buildUpdatePayloadFromForm(form);
      if (result.error) {
        setWorkoutMessage(result.error, true);
        return;
      }
      const payload = result.payload;

      await app.api(`/api/workouts/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      }, true);

      setWorkoutMessage('Тренировка обновлена.');
      await openWorkoutDetails(id);
      if (filterMode) {
        await loadWorkouts();
      } else {
        await resetAndLoadHistory();
      }
    } catch (error) {
      setWorkoutMessage(error.message, true);
    }
  });
}

function bindDelete() {
  async function localConfirm(message, options = {}) {
    return new Promise((resolve) => {
      const overlay = document.createElement('div');
      overlay.className = 'app-dialog-overlay';
      overlay.innerHTML = `
        <div class="app-dialog" role="dialog" aria-modal="true" aria-labelledby="workout-local-dialog-title">
          <h3 id="workout-local-dialog-title" class="app-dialog-title"></h3>
          <p class="app-dialog-message"></p>
          <div class="app-dialog-actions">
            <button type="button" class="btn btn-outline stable-btn workout-local-dialog-cancel">Отмена</button>
            <button type="button" class="btn btn-primary stable-btn workout-local-dialog-confirm">Подтвердить</button>
          </div>
        </div>
      `;

      const title = overlay.querySelector('.app-dialog-title');
      const text = overlay.querySelector('.app-dialog-message');
      const cancel = overlay.querySelector('.workout-local-dialog-cancel');
      const confirm = overlay.querySelector('.workout-local-dialog-confirm');

      if (title) {
        title.textContent = options.title || 'Подтверждение';
      }
      if (text) {
        text.textContent = message || '';
      }
      if (cancel) {
        cancel.textContent = options.cancelText || 'Отмена';
      }
      if (confirm) {
        confirm.textContent = options.confirmText || 'Подтвердить';
      }

      const close = (value) => {
        overlay.remove();
        resolve(value);
      };

      cancel?.addEventListener('click', () => close(false));
      confirm?.addEventListener('click', () => close(true));
      overlay.addEventListener('click', (event) => {
        if (event.target === overlay) {
          close(false);
        }
      });

      const escHandler = (event) => {
        if (event.key === 'Escape') {
          document.removeEventListener('keydown', escHandler);
          close(false);
        }
      };

      document.addEventListener('keydown', escHandler);
      document.body.appendChild(overlay);
    });
  }

  const app = window.SmartRunApp;
  const btn = document.getElementById('workout-delete-btn');
  btn?.addEventListener('click', async () => {
    const id = Number(document.getElementById('workout-edit-id')?.value || selectedWorkoutId);
    if (!Number.isFinite(id) || id <= 0) {
      setWorkoutMessage('Сначала выберите тренировку.', true);
      return;
    }

    const confirmed = typeof app?.confirm === 'function'
      ? await app.confirm('Удалить эту тренировку? Действие необратимо.', {
          title: 'Подтвердите удаление',
          confirmText: 'Удалить',
          cancelText: 'Отмена'
        })
      : await localConfirm('Удалить эту тренировку? Действие необратимо.', {
          title: 'Подтвердите удаление',
          confirmText: 'Удалить',
          cancelText: 'Отмена'
        });

    if (!confirmed) {
      return;
    }

    try {
      await app.api(`/api/workouts/${id}`, { method: 'DELETE' }, true);

      if (isWorkoutDetailsRoute()) {
        window.location.href = '/workouts';
        return;
      }

      closeDetailsPanel();
      setWorkoutMessage('Тренировка удалена.');
      if (filterMode) {
        await loadWorkouts();
      } else {
        await resetAndLoadHistory();
      }
    } catch (error) {
      setWorkoutMessage(error.message, true);
    }
  });
}

function bindDetailsClose() {
  const closeBtn = document.getElementById('workout-details-close');
  closeBtn?.addEventListener('click', () => {
    if (isWorkoutDetailsRoute()) {
      window.location.href = '/workouts';
      return;
    }
    closeDetailsPanel();
  });
}

document.addEventListener('smartrun:ready', (event) => {
  const hrZoneFields = [
    'zone_1_seconds',
    'zone_2_seconds',
    'zone_3_seconds',
    'zone_4_seconds',
    'zone_5_seconds',
    'zone_6_seconds'
  ];

  restoreWorkoutsScreenState(event.detail?.user || null);
  applyDetailsRouteMode();
  applyDateLimits();
  setFilterMode(filterMode);
  bindFilters();
  bindQuickFilters();
  bindHistory();
  bindHistoryInfiniteScroll();
  bindDurationAutoFormat('workout-create-form', 'duration');
  bindDurationAutoFormat('workout-edit-form', 'duration');
  bindDurationAutoFormatMany('workout-create-form', hrZoneFields, true);
  bindDurationAutoFormatMany('workout-edit-form', hrZoneFields, true);
  bindCreatePanel();
  bindCreateForm();
  bindFitImport();
  bindListActions();
  bindEditForm();
  bindDelete();
  bindDetailsClose();

  const detailsId = getWorkoutIdFromPath();
  if (detailsId != null) {
    openWorkoutDetails(detailsId);
  } else {
    if (filterMode) {
      loadWorkouts();
    } else {
      resetAndLoadHistory();
    }
  }
});
