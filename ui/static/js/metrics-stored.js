function msMessage(text, isError = false) {
  const node = document.getElementById('ms-message');
  if (!node) {
    return;
  }
  node.textContent = text || '';
  node.classList.toggle('error', Boolean(isError));
}

function msNumber(value, fallback = 0) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function msFormatPace(value) {
  const num = Number(value);
  if (!Number.isFinite(num)) {
    return '-';
  }
  const m = Math.floor(num);
  const s = Math.round((num - m) * 60);
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')} /км`;
}

function msFormatDuration(seconds) {
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

function msApplySummary(metric) {
  const summary = document.getElementById('ms-summary');
  const empty = document.getElementById('ms-empty');
  if (!summary || !empty) {
    return;
  }

  if (!metric) {
    summary.classList.add('hidden');
    empty.classList.remove('hidden');
    return;
  }

  empty.classList.add('hidden');
  summary.classList.remove('hidden');

  document.getElementById('ms-id').textContent = String(metric.id ?? '-');
  document.getElementById('ms-period').textContent = `${metric.from || '-'} — ${metric.to || '-'}`;
  document.getElementById('ms-workouts').textContent = String(metric.total_workouts ?? 0);
  document.getElementById('ms-distance').textContent = msNumber(metric.total_distance, 0).toFixed(1);
  document.getElementById('ms-duration').textContent = msFormatDuration(metric.total_duration);
  document.getElementById('ms-pace').textContent = msFormatPace(metric.avg_pace);
  document.getElementById('ms-calories').textContent = String(Math.round(msNumber(metric.total_calories, 0)));
}

function msFillForm(metric) {
  document.getElementById('ms-form-id').value = metric?.id ?? '';
  document.getElementById('ms-form-from').value = String(metric?.from || '').slice(0, 10);
  document.getElementById('ms-form-to').value = String(metric?.to || '').slice(0, 10);
  document.getElementById('ms-form-workouts').value = metric?.total_workouts ?? 0;
  document.getElementById('ms-form-distance').value = metric?.total_distance ?? 0;
  document.getElementById('ms-form-duration').value = metric?.total_duration ?? 0;
  document.getElementById('ms-form-pace').value = metric?.avg_pace ?? 0;
  document.getElementById('ms-form-calories').value = metric?.total_calories ?? 0;
}

async function msLoadStored() {
  const app = window.SmartRunApp;
  try {
    const metric = await app.api('/api/metrics/stored', { method: 'GET' }, true);
    msApplySummary(metric);
    msFillForm(metric);
    return metric;
  } catch (error) {
    const message = String(error?.message || 'Не удалось загрузить snapshot');
    if (message.includes('404')) {
      msApplySummary(null);
      msFillForm(null);
      msMessage('Снимок пока не создан');
      return null;
    }
    msMessage(message, true);
    msApplySummary(null);
    return null;
  }
}

document.addEventListener('smartrun:ready', () => {
  const app = window.SmartRunApp;
  let stored = null;

  const form = document.getElementById('ms-form');

  document.getElementById('ms-refresh')?.addEventListener('click', async () => {
    msMessage('Обновляем...');
    stored = await msLoadStored();
  });

  form?.addEventListener('submit', async (event) => {
    event.preventDefault();

    const fd = new FormData(form);
    const payload = {
      total_workouts: msNumber(fd.get('total_workouts'), 0),
      total_distance: msNumber(fd.get('total_distance'), 0),
      total_duration: msNumber(fd.get('total_duration'), 0),
      avg_pace: msNumber(fd.get('avg_pace'), 0),
      total_calories: msNumber(fd.get('total_calories'), 0),
      from: String(fd.get('from') || ''),
      to: String(fd.get('to') || '')
    };

    try {
      msMessage('Сохраняем...');
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

      stored = await msLoadStored();
      msMessage('Snapshot сохранен');
    } catch (error) {
      msMessage(String(error?.message || 'Не удалось сохранить snapshot'), true);
    }
  });

  document.getElementById('ms-delete-btn')?.addEventListener('click', async () => {
    if (!stored?.id) {
      msMessage('Нет snapshot для удаления', true);
      return;
    }

    const confirmed = await app.confirm('Удалить текущий snapshot?', {
      title: 'Удаление snapshot',
      confirmText: 'Удалить'
    });
    if (!confirmed) {
      return;
    }

    try {
      msMessage('Удаляем...');
      await app.api(`/api/metrics/${stored.id}`, { method: 'DELETE' }, true);
      stored = null;
      msApplySummary(null);
      msFillForm(null);
      msMessage('Snapshot удален');
    } catch (error) {
      msMessage(String(error?.message || 'Не удалось удалить snapshot'), true);
    }
  });

  msLoadStored().then((metric) => {
    stored = metric;
  });
});
