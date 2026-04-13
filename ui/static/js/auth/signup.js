const signupForm = document.getElementById('signup-form');
const signupMessage = document.getElementById('signup-message');

function parseOptionalNumber(value) {
  if (value === '' || value == null) {
    return null;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}

  function paceTextToMinutes(text) {
    const match = /^([0-9]{1,2}):([0-5][0-9])$/.exec(String(text || '').trim());
    if (!match) {
      return null;
    }
    const minutes = Number(match[1]);
    const seconds = Number(match[2]);
    return minutes + seconds / 60;
  }

if (signupForm && signupMessage) {
  signupForm.addEventListener('submit', async (event) => {
    event.preventDefault();
    signupMessage.textContent = 'Создаем аккаунт...';
    signupMessage.classList.remove('error');

    const formData = new FormData(signupForm);
    const payload = {
      name: String(formData.get('name') || '').trim(),
      email: String(formData.get('email') || '').trim(),
      password: String(formData.get('password') || '')
    };

    const gender = String(formData.get('gender') || '').trim();
    if (gender) {
      payload.gender = gender;
    }

    const numericFields = [
      'age',
      'weight_kg',
      'height_cm',
      'resting_hr',
      'max_hr',
      'weekly_runs'
    ];

    numericFields.forEach((field) => {
      const parsed = parseOptionalNumber(formData.get(field));
      if (parsed !== null) {
        payload[field] = parsed;
      }
    });

    const paceText = String(formData.get('threshold_pace_text') || '').trim();
    if (paceText) {
      const paceMinutes = paceTextToMinutes(paceText);
      if (paceMinutes == null) {
        signupMessage.textContent = 'Пороговый темп нужно указать как мм:сс, например 04:30.';
        signupMessage.classList.add('error');
        return;
      }
      payload.threshold_pace_min_km = paceMinutes;
    }

    try {
      const response = await fetch('/api/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(payload)
      });

      const text = await response.text();
      let data = null;

      try {
        data = text ? JSON.parse(text) : null;
      } catch (_) {}

      if (!response.ok) {
        const message = data?.message || text || 'Ошибка регистрации';
        signupMessage.textContent = message;
        signupMessage.classList.add('error');
        return;
      }

      if (data?.auth?.access_token) {
        if (window.SmartRunApp?.setToken) {
          window.SmartRunApp.setToken(data.auth.access_token);
        } else {
          localStorage.setItem('access_token', data.auth.access_token);
        }
      }

      signupMessage.textContent = 'Регистрация успешна. Переходим в dashboard...';
      setTimeout(() => {
        window.location.href = '/dashboard';
      }, 400);
    } catch (error) {
      signupMessage.textContent = 'Сеть недоступна. Попробуйте еще раз.';
      signupMessage.classList.add('error');
    }
  });
}