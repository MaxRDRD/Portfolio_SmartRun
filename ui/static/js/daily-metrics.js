function dmSetMessage(text, isError = false) {
  const node = document.getElementById('dm-message');
  if (!node) {
    return;
  }
  node.textContent = text || '';
  node.classList.toggle('error', Boolean(isError));
}

function dmNumber(value, fallback = 0) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function dmToIsoDateTime(dateYmd) {
  if (!dateYmd) {
    return new Date().toISOString();
  }
  return `${dateYmd}T00:00:00Z`;
}

function dmRenderList(metrics) {
  const list = document.getElementById('dm-list');
  const empty = document.getElementById('dm-empty');
  if (!list || !empty) {
    return;
  }

  if (!Array.isArray(metrics) || metrics.length === 0) {
    list.innerHTML = '';
    empty.classList.remove('hidden');
    return;
  }

  empty.classList.add('hidden');

  const sorted = [...metrics].sort((a, b) => String(b.date).localeCompare(String(a.date)));
  list.innerHTML = sorted.map((item) => {
    const readiness = item.readiness_score ?? '-';
    const recommendation = item.recommendation || 'Без рекомендации';
    return `<div class="list-item card-row">
      <div class="workout-item-main">
        <strong>${item.date || '-'}</strong>
        <p>Readiness: ${readiness} • CTL: ${dmNumber(item.ctl, 0).toFixed(1)} • ATL: ${dmNumber(item.atl, 0).toFixed(1)} • Steps: ${dmNumber(item.steps, 0)}</p>
        <p>${recommendation}</p>
      </div>
      <div class="workout-item-actions">
        <button class="btn btn-outline btn-sm dm-edit-btn" type="button" data-id="${item.id}">Редактировать</button>
      </div>
    </div>`;
  }).join('');
}

function dmFillEditForm(metric) {
  if (!metric) {
    return;
  }

  document.getElementById('dm-edit-id').value = metric.id ?? '';
  document.getElementById('dm-edit-date').value = String(metric.date || '').slice(0, 10);
  document.getElementById('dm-edit-ctl').value = metric.ctl ?? 0;
  document.getElementById('dm-edit-atl').value = metric.atl ?? 0;
  document.getElementById('dm-edit-tsb').value = metric.tsb ?? 0;
  document.getElementById('dm-edit-fatigue').value = metric.fatigue_score ?? 0;
  document.getElementById('dm-edit-readiness').value = metric.readiness_score ?? 0;
  document.getElementById('dm-edit-body-battery').value = metric.body_battery_avg ?? 0;
  document.getElementById('dm-edit-steps').value = metric.steps ?? 0;
  document.getElementById('dm-edit-total-calories').value = metric.total_calories ?? 0;
  document.getElementById('dm-edit-sleep').value = metric.sleep_score ?? 0;
  document.getElementById('dm-edit-stress').value = metric.stress_avg ?? 0;
  document.getElementById('dm-edit-streak').value = metric.streak_days ?? 0;
  document.getElementById('dm-edit-monotony').value = metric.monotony ?? 0;
  document.getElementById('dm-edit-strain').value = metric.strain ?? '';
  document.getElementById('dm-edit-recommendation').value = metric.recommendation ?? '';
}

function dmToggleCreatePanel(show) {
  document.getElementById('dm-create-panel')?.classList.toggle('hidden', !show);
}

function dmToggleEditPanel(show) {
  document.getElementById('dm-edit-panel')?.classList.toggle('hidden', !show);
}

async function dmLoadAll() {
  const app = window.SmartRunApp;
  try {
    const metrics = await app.api('/api/daily-metrics', { method: 'GET' }, true);
    dmRenderList(metrics);
    return Array.isArray(metrics) ? metrics : [];
  } catch (error) {
    const message = String(error?.message || 'Не удалось загрузить daily metrics');
    if (message.includes('404')) {
      dmRenderList([]);
      return [];
    }
    dmSetMessage(message, true);
    dmRenderList([]);
    return [];
  }
}

document.addEventListener('smartrun:ready', () => {
  const app = window.SmartRunApp;
  let cache = [];
  let selectedMetric = null;

  const createForm = document.getElementById('dm-create-form');
  const editForm = document.getElementById('dm-edit-form');
  const listNode = document.getElementById('dm-list');

  document.getElementById('dm-open-create')?.addEventListener('click', () => {
    dmToggleCreatePanel(true);
    dmToggleEditPanel(false);
    dmSetMessage('');
  });

  document.getElementById('dm-create-cancel')?.addEventListener('click', () => {
    dmToggleCreatePanel(false);
  });

  document.getElementById('dm-edit-close')?.addEventListener('click', () => {
    dmToggleEditPanel(false);
  });

  document.getElementById('dm-refresh')?.addEventListener('click', async () => {
    dmSetMessage('Обновляем...');
    cache = await dmLoadAll();
    dmSetMessage('Обновлено');
  });

  listNode?.addEventListener('click', (event) => {
    const target = event.target;
    if (!(target instanceof HTMLElement)) {
      return;
    }

    const editBtn = target.closest('.dm-edit-btn');
    if (!editBtn) {
      return;
    }

    const id = Number(editBtn.getAttribute('data-id'));
    const metric = cache.find((item) => Number(item?.id) === id);
    if (!metric) {
      dmSetMessage('Запись не найдена', true);
      return;
    }

    selectedMetric = metric;
    dmFillEditForm(metric);
    dmToggleEditPanel(true);
    dmToggleCreatePanel(false);
    dmSetMessage('');
  });

  createForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
    dmSetMessage('Сохраняем...');

    const fd = new FormData(createForm);
    const payload = {
      date: String(fd.get('date') || ''),
      sleep_score: dmNumber(fd.get('sleep_score'), 0),
      body_battery_avg: dmNumber(fd.get('body_battery_avg'), 0),
      steps: dmNumber(fd.get('steps'), 0)
    };

    try {
      await app.api('/api/daily-metrics', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      }, true);
      createForm.reset();
      dmToggleCreatePanel(false);
      cache = await dmLoadAll();
      dmSetMessage('Запись создана');
    } catch (error) {
      dmSetMessage(String(error?.message || 'Не удалось создать запись'), true);
    }
  });

  editForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
    if (!selectedMetric) {
      dmSetMessage('Сначала выберите запись', true);
      return;
    }

    dmSetMessage('Сохраняем изменения...');

    const fd = new FormData(editForm);
    const id = dmNumber(fd.get('id'), 0);
    const strainRaw = String(fd.get('strain') || '').trim();

    const payload = {
      id,
      date: dmToIsoDateTime(String(fd.get('date') || '')),
      ctl: dmNumber(fd.get('ctl'), 0),
      atl: dmNumber(fd.get('atl'), 0),
      tsb: dmNumber(fd.get('tsb'), 0),
      fatigue_score: dmNumber(fd.get('fatigue_score'), 0),
      readiness_score: dmNumber(fd.get('readiness_score'), 0),
      body_battery_avg: dmNumber(fd.get('body_battery_avg'), 0),
      steps: dmNumber(fd.get('steps'), 0),
      total_calories: dmNumber(fd.get('total_calories'), 0),
      sleep_score: dmNumber(fd.get('sleep_score'), 0),
      stress_avg: dmNumber(fd.get('stress_avg'), 0),
      recommendation: String(fd.get('recommendation') || ''),
      streak_days: dmNumber(fd.get('streak_days'), 0),
      monotony: dmNumber(fd.get('monotony'), 0),
      strain: strainRaw === '' ? null : dmNumber(strainRaw, 0)
    };

    try {
      await app.api(`/api/daily-metrics/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      }, true);
      cache = await dmLoadAll();
      selectedMetric = cache.find((item) => Number(item?.id) === id) || null;
      if (selectedMetric) {
        dmFillEditForm(selectedMetric);
      }
      dmSetMessage('Изменения сохранены');
    } catch (error) {
      dmSetMessage(String(error?.message || 'Не удалось обновить запись'), true);
    }
  });

  document.getElementById('dm-delete-btn')?.addEventListener('click', async () => {
    if (!selectedMetric) {
      dmSetMessage('Сначала выберите запись', true);
      return;
    }

    const confirmed = await app.confirm('Удалить выбранную daily metrics запись?', {
      title: 'Удаление записи',
      confirmText: 'Удалить'
    });
    if (!confirmed) {
      return;
    }

    dmSetMessage('Удаляем...');
    try {
      await app.api(`/api/daily-metrics/${selectedMetric.id}`, { method: 'DELETE' }, true);
      selectedMetric = null;
      dmToggleEditPanel(false);
      cache = await dmLoadAll();
      dmSetMessage('Запись удалена');
    } catch (error) {
      dmSetMessage(String(error?.message || 'Не удалось удалить запись'), true);
    }
  });

  dmLoadAll().then((items) => {
    cache = items;
  });
});
