function maMessage(text, isError = false) {
  const node = document.getElementById('ma-message');
  if (!node) {
    return;
  }
  node.textContent = text || '';
  node.classList.toggle('error', Boolean(isError));
}

function maFormatPace(value) {
  const num = Number(value);
  if (!Number.isFinite(num)) {
    return '-';
  }
  const m = Math.floor(num);
  const s = Math.round((num - m) * 60);
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')} /км`;
}

function maFormatDuration(seconds) {
  const total = Number(seconds || 0);
  if (!Number.isFinite(total) || total < 0) {
    return '-';
  }
  const whole = Math.floor(total);
  const hours = Math.floor(whole / 3600);
  const minutes = Math.floor((whole % 3600) / 60);
  const secs = whole % 60;
  return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
}

function maTodayISO() {
  return new Date().toISOString().slice(0, 10);
}

function maMonthStartISO() {
  const now = new Date();
  const first = new Date(now.getFullYear(), now.getMonth(), 1);
  return first.toISOString().slice(0, 10);
}

function maApplyResult(metric) {
  const result = document.getElementById('ma-result');
  const empty = document.getElementById('ma-result-empty');
  if (!result || !empty) {
    return;
  }

  if (!metric) {
    result.classList.add('hidden');
    empty.classList.remove('hidden');
    return;
  }

  empty.classList.add('hidden');
  result.classList.remove('hidden');

  document.getElementById('ma-period').textContent = `${metric.from || '-'} — ${metric.to || '-'}`;
  document.getElementById('ma-workouts').textContent = String(metric.total_workouts ?? 0);
  document.getElementById('ma-distance').textContent = Number(metric.total_distance ?? 0).toFixed(1);
  document.getElementById('ma-duration').textContent = maFormatDuration(metric.total_duration);
  document.getElementById('ma-pace').textContent = maFormatPace(metric.avg_pace);
  document.getElementById('ma-calories').textContent = String(Math.round(Number(metric.total_calories ?? 0)));
}

document.addEventListener('smartrun:ready', () => {
  const app = window.SmartRunApp;
  let currentMetric = null;

  const form = document.getElementById('ma-range-form');
  const fromInput = document.getElementById('ma-from');
  const toInput = document.getElementById('ma-to');

  fromInput.value = maMonthStartISO();
  toInput.value = maTodayISO();

  async function calculateRange(from, to) {
    maMessage('Считаем...');
    try {
      const metric = await app.api(`/api/metrics?from=${from}&to=${to}`, { method: 'GET' }, true);
      currentMetric = metric;
      maApplyResult(metric);
      maMessage('Расчет готов');
    } catch (error) {
      currentMetric = null;
      maApplyResult(null);
      maMessage(String(error?.message || 'Не удалось рассчитать метрики'), true);
    }
  }

  form?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const from = String(fromInput.value || '');
    const to = String(toInput.value || '');
    if (!from || !to) {
      maMessage('Укажите обе даты', true);
      return;
    }
    await calculateRange(from, to);
  });

  document.getElementById('ma-all-time')?.addEventListener('click', async () => {
    fromInput.value = '1970-01-01';
    toInput.value = maTodayISO();
    await calculateRange(fromInput.value, toInput.value);
  });

  document.getElementById('ma-save-snapshot')?.addEventListener('click', async () => {
    if (!currentMetric) {
      maMessage('Сначала выполните расчет диапазона', true);
      return;
    }

    const payload = {
      total_workouts: Number(currentMetric.total_workouts ?? 0),
      total_distance: Number(currentMetric.total_distance ?? 0),
      total_duration: Number(currentMetric.total_duration ?? 0),
      avg_pace: Number(currentMetric.avg_pace ?? 0),
      total_calories: Number(currentMetric.total_calories ?? 0),
      from: currentMetric.from || fromInput.value,
      to: currentMetric.to || toInput.value
    };

    maMessage('Сохраняем snapshot...');
    try {
      const stored = await app.api('/api/metrics/stored', { method: 'GET' }, true);
      if (stored?.id) {
        await app.api(`/api/metrics/${stored.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        }, true);
      } else {
        await app.api('/api/metrics', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        }, true);
      }
    } catch (error) {
      const message = String(error?.message || 'save failed');
      if (message.includes('404')) {
        try {
          await app.api('/api/metrics', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
          }, true);
        } catch (postError) {
          maMessage(String(postError?.message || 'Не удалось сохранить snapshot'), true);
          return;
        }
      } else {
        maMessage(message, true);
        return;
      }
    }

    maMessage('Snapshot сохранен');
  });

  calculateRange(fromInput.value, toInput.value);
});
