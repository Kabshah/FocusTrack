const APP_COLORS = [
  '#e8c87a', /* amber      */
  '#c87a5a', /* terracotta */
  '#7ab87a', /* sage       */
  '#a87ac8', /* lavender   */
  '#7aaac8', /* steel      */
  '#c8a87a', /* sand       */
  '#7ac8b8', /* muted teal */
  '#c87aa8', /* dusty rose */
];

const DAY_COLORS = [
  '#e8c87a', // Mon — amber
  '#c87a5a', // Tue — terracotta
  '#7ab87a', // Wed — sage green
  '#a87ac8', // Thu — lavender
  '#7aaac8', // Fri — steel blue
  '#c8a87a', // Sat — sand
  '#c87aa8', // Sun — dusty rose
];

let selectedDay = null;
let currentWeeklySortedApps = [];

function formatDuration(totalSec) {
  if (!totalSec || totalSec < 60) return '0m';
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  if (h === 0) return `${m}m`;
  if (m === 0) return `${h}h`;
  return `${h}h ${m}m`;
}

// Tab switching
document.querySelectorAll('.tab').forEach(tab => {
  tab.addEventListener('click', () => {
    document.querySelectorAll('.tab, .content').forEach(el => el.classList.remove('active'));
    tab.classList.add('active');
    document.getElementById(tab.dataset.tab).classList.add('active');
    if (tab.dataset.tab === 'today') loadToday();
    if (tab.dataset.tab === 'week') {
        selectedDay = null;
        clearDaySelection();
        loadWeek();
    }
    if (tab.dataset.tab === 'settings') loadLimitsList();
  });
});

async function saveToggle(key, value) {
    await fetch('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ [key]: value })
    });
}

function toggleState(id) {
    const el = document.getElementById(id);
    if (!el) return false;
    const isOn = el.dataset.on === 'true';
    el.dataset.on = (!isOn).toString();
    return !isOn;
}

document.getElementById('toggle-startup').addEventListener('click', async () => {
    const isOn = toggleState('toggle-startup');
    await saveToggle('startWithWindows', isOn);
});

document.getElementById('toggle-notify').addEventListener('click', async () => {
    const isOn = toggleState('toggle-notify');
    await saveToggle('notifyDaily', isOn);
});

document.getElementById('toggle-limits').addEventListener('click', async () => {
    const isOn = toggleState('toggle-limits');
    await saveToggle('limitAlerts', isOn);
});

function setToggle(id, isOn) {
    const el = document.getElementById(id);
    if (!el) return;
    el.dataset.on = isOn.toString();
}

async function loadSettings() {
    try {
        const res = await fetch('/api/settings');
        const cfg = await res.json();
        setToggle('toggle-startup', cfg.startWithWindows ?? false);
        setToggle('toggle-notify',  cfg.notifyDaily      ?? false);
        setToggle('toggle-limits',  cfg.limitAlerts      ?? true);
    } catch (e) {
        console.warn('Failed to load settings', e);
    }
}

// Makes color consistent for same app name across all views
function appColorByName(name) {
    let hash = 0;
    for (let i = 0; i < name.length; i++) {
        hash = name.charCodeAt(i) + ((hash << 5) - hash);
    }
    return APP_COLORS[Math.abs(hash) % APP_COLORS.length];
}

async function loadLimitsList() {
    try {
        const res = await fetch('/api/limits');
        const limits = await res.json();
        const container = document.getElementById('limits-list');
        if (!container) return;
        container.innerHTML = '';

        const entries = Object.entries(limits);
        if (entries.length === 0) {
            container.innerHTML = '<div class="empty-text" style="padding:12px 18px;text-align:left;">No limits set.</div>';
            return;
        }

        entries.forEach(([appName, minutes]) => {
            const color = appColorByName(appName);
            container.insertAdjacentHTML('beforeend', `
                <div class="limit-item" data-app="${appName}">
                    <div class="limit-left">
                        <div class="app-dot" style="background:${color}"></div>
                        <div>
                            <div class="limit-name">${appName}</div>
                            <div class="limit-sub">${minutes} min / day</div>
                        </div>
                    </div>
                    <button class="limit-delete-btn" onclick="deleteLimit('${appName}')">Remove</button>
                </div>`);
        });
    } catch (e) {
        console.warn('Failed to load limits', e);
    }
}

async function deleteLimit(appName) {
    await fetch('/api/limits', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ app: appName })
    });
    loadLimitsList();
}

async function loadToday() {
  if (!window.getDailyStats) return;
  const raw = await window.getDailyStats();
  const data = JSON.parse(raw);
  
  const apps = data.Apps || [];
  const totalSec = apps.reduce((acc, app) => acc + app.TotalSeconds, 0);
  
  document.getElementById('today-total').textContent = formatDuration(totalSec);

  const pctOfDay = (totalSec / 86400) * 100;
  const pctDisplay = pctOfDay.toFixed(1) + '%';
  const circumference = 201;
  const offset = circumference - (circumference * Math.min(pctOfDay / 100, 1));

  document.getElementById('ring-pct').textContent = pctDisplay;
  document.getElementById('ring-fill').style.strokeDashoffset = offset.toFixed(1);

  const appCount = apps.length;
  const dateStr = new Date().toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short' });
  document.getElementById('today-sub').textContent = `${dateStr} · ${appCount} apps used`;

  const listEl = document.getElementById('app-list');
  listEl.innerHTML = '';

  const topSec = apps.length > 0 ? apps[0].TotalSeconds : 0;

  apps.slice(0, 10).forEach((app, i) => {
    const color = APP_COLORS[i % APP_COLORS.length];
    const pct = topSec > 0 ? (app.TotalSeconds / topSec * 100) : 0;
    
    let sessionsHtml = '';
    if (app.Sessions !== undefined) {
      sessionsHtml = `<span class="app-sessions">${app.Sessions} session${app.Sessions !== 1 ? 's' : ''}</span>`;
    } else if (app.RunCount !== undefined) {
      sessionsHtml = `<span class="app-sessions">${app.RunCount} session${app.RunCount !== 1 ? 's' : ''}</span>`;
    }
    
    const row = `
      <div class="app-row">
        <div class="app-row-top">
          <div class="app-row-left">
            <div class="app-dot" style="background:${color}"></div>
            <span class="app-name">${app.AppName}</span>
            ${sessionsHtml}
          </div>
          <span class="app-time">${formatDuration(app.TotalSeconds)}</span>
        </div>
        <div class="progress-track">
          <div class="progress-fill" style="width:${pct.toFixed(1)}%;background:${color}"></div>
        </div>
      </div>`;
    listEl.insertAdjacentHTML('beforeend', row);
  });
}

function renderWeeklyAppList() {
  const container = document.getElementById('week-day-detail');
  let appsHTML = '';
  
  const weekTopAppSec = currentWeeklySortedApps.length > 0 ? currentWeeklySortedApps[0][1] : 0;
  currentWeeklySortedApps.slice(0, 10).forEach(([name, secs], i) => {
    const color = APP_COLORS[i % APP_COLORS.length];
    const pct = weekTopAppSec > 0 ? (secs / weekTopAppSec * 100) : 0;
    
    appsHTML += `
      <div class="app-row">
        <div class="app-row-top">
          <div class="app-row-left">
            <div class="app-dot" style="background:${color}"></div>
            <span class="app-name">${name}</span>
          </div>
          <span class="app-time">${formatDuration(secs)}</span>
        </div>
        <div class="progress-track">
          <div class="progress-fill" style="width:${pct.toFixed(1)}%;background:${color}"></div>
        </div>
      </div>`;
  });

  container.innerHTML = '<div class="app-list">' + appsHTML + '</div>';
}

async function loadWeek() {
  if (!window.getWeeklyStats) return;
  const raw = await window.getWeeklyStats();
  const data = JSON.parse(raw); 

  const daysData = data.Days || [];
  const weeklyTotalSec = daysData.reduce((acc, d) => acc + d.TotalSeconds, 0);
  
  const weekPct = (weeklyTotalSec / 604800) * 100;
  document.getElementById('week-pct').textContent = weekPct.toFixed(2) + '%';
  document.getElementById('week-total').textContent = formatDuration(weeklyTotalSec);

  const chartEl = document.getElementById('week-chart');
  chartEl.innerHTML = '';
  
  let days = [];
  if (daysData.length > 0) {
    days = daysData.map(d => {
      const dateObj = new Date(d.Date);
      return {
        isoDate: d.Date.split('T')[0],
        totalSec: d.TotalSeconds,
        label: dateObj.toLocaleDateString('en-GB', { weekday: 'short' }).slice(0, 3)
      };
    });
  } else {
    for (let i = 6; i >= 0; i--) {
      const d = new Date();
      d.setDate(d.getDate() - i);
      const iso = d.toISOString().split('T')[0];
      days.push({
        isoDate: iso,
        totalSec: 0,
        label: d.toLocaleDateString('en-GB', { weekday: 'short' }).slice(0, 3)
      });
    }
  }

  const maxSec = Math.max(...days.map(d => d.totalSec), 1);
  const todayLabel = new Date().toLocaleDateString('en-GB', { weekday: 'short' }).slice(0, 3);

  days.forEach((day, i) => {
    const isToday = day.label === todayLabel;
    const barHeight = Math.max((day.totalSec / maxSec) * 80, 3);
    const barColor = DAY_COLORS[i % DAY_COLORS.length];
    const opacity = day.totalSec > 0 ? 0.85 : 0.15;

    chartEl.insertAdjacentHTML('beforeend', `
      <div class="bar-col" data-date="${day.isoDate}" style="cursor: pointer;" onclick="selectDay('${day.isoDate}', '${day.label}')">
        <div class="bar-val">${day.totalSec > 0 ? formatDuration(day.totalSec) : ''}</div>
        <div class="bar" style="height:${barHeight}px;background:${barColor};opacity:${isToday ? 1 : opacity}"></div>
        <div class="bar-day ${isToday ? 'today' : ''}">${day.label}</div>
      </div>`);
  });

  const avgSec = Math.floor(weeklyTotalSec / 7);
  document.getElementById('week-avg').textContent = formatDuration(avgSec);
  
  const activeDays = days.filter(d => d.totalSec > 0).length;
  document.getElementById('week-active-days').innerHTML = `${activeDays} <span class="stat-pill-denom">/ 7</span>`;

  const appMap = {};
  daysData.forEach(day => {
    if (day.TopApps) {
      day.TopApps.forEach(app => {
        appMap[app.AppName] = (appMap[app.AppName] || 0) + app.TotalSeconds;
      });
    }
  });

  currentWeeklySortedApps = Object.entries(appMap).sort((a, b) => b[1] - a[1]);
  if (currentWeeklySortedApps.length > 0) {
    document.getElementById('week-top-app').textContent = currentWeeklySortedApps[0][0];
    document.getElementById('week-top-app-time').textContent = `${formatDuration(currentWeeklySortedApps[0][1])} this week`;
  } else {
    document.getElementById('week-top-app').textContent = '—';
    document.getElementById('week-top-app-time').textContent = '0m this week';
  }

  if (selectedDay == null) {
      renderWeeklyAppList();
  }
}

async function selectDay(isoDate, dayLabel) {
    selectedDay = isoDate;

    // Highlight selected bar, dim others
    document.querySelectorAll('.bar-col').forEach(col => {
        const isSelected = col.dataset.date === isoDate;
        col.style.opacity = isSelected ? '1' : '0.35';
    });

    // Show back button
    document.getElementById('week-back').style.display = 'flex';

    // Update section label
    document.getElementById('week-section-label').textContent = formatDayLabel(isoDate);

    // Fetch and render day data
    await loadDayDetail(isoDate);
}

function clearDaySelection() {
    selectedDay = null;

    // Reset all bars to full opacity
    document.querySelectorAll('.bar-col').forEach(col => {
        col.style.opacity = '1';
    });

    // Hide back button
    const backBtn = document.getElementById('week-back');
    if(backBtn) backBtn.style.display = 'none';

    // Reset section label
    const sectionLabel = document.getElementById('week-section-label');
    if(sectionLabel) sectionLabel.textContent = 'WEEKLY APP USAGE';

    // Show weekly app list again
    renderWeeklyAppList(); 
}

async function loadDayDetail(isoDate) {
    const container = document.getElementById('week-day-detail');
    container.innerHTML = '<div class="loading-text">Loading...</div>';

    try {
        const res  = await fetch(`/api/day?date=${isoDate}`);
        const data = await res.json();

        // ── Day hero card ─────────────────────────────────────────
        const pctOfDay    = ((data.totalSec / 86400) * 100).toFixed(1);
        const circumference = 201; // 2 * π * 32
        const ringOffset  = (circumference - (circumference * Math.min(data.totalSec / 86400, 1))).toFixed(1);

        const heroHTML = `
            <div class="card hero-card day-hero-card">
                <div class="hero-left">
                    <div class="hero-label">${formatDayHeading(isoDate)}</div>
                    <div class="hero-time">${formatDuration(data.totalSec)}</div>
                    <div class="hero-sub">${data.apps ? data.apps.length : 0} apps used</div>
                </div>
                <div class="hero-ring">
                    <svg width="80" height="80" viewBox="0 0 80 80">
                        <circle cx="40" cy="40" r="32"
                            fill="none" stroke="#1e1e22" stroke-width="6"/>
                        <circle cx="40" cy="40" r="32"
                            fill="none" stroke="#e8c87a" stroke-width="6"
                            stroke-dasharray="${circumference}"
                            stroke-dashoffset="${ringOffset}"
                            stroke-linecap="round"
                            style="transform:rotate(-90deg);transform-origin:center;
                                   transition:stroke-dashoffset 0.6s ease;"/>
                    </svg>
                    <div class="ring-label">
                        <span>${pctOfDay}%</span>
                        <span>of day</span>
                    </div>
                </div>
            </div>`;

        // ── Apps list ─────────────────────────────────────────────
        let appsHTML = '';
        if (!data.apps || data.apps.length === 0) {
            appsHTML = '<div class="empty-text">No activity recorded on this day.</div>';
        } else {
            data.apps.forEach((app, i) => {
                const color = APP_COLORS[i % APP_COLORS.length];
                appsHTML += `
                    <div class="app-row">
                        <div class="app-row-top">
                            <div class="app-row-left">
                                <div class="app-dot" style="background:${color}"></div>
                                <span class="app-name">${app.name}</span>
                                <span class="app-sessions">${app.sessions} session${app.sessions !== 1 ? 's' : ''}</span>
                            </div>
                            <span class="app-time">${formatDuration(app.totalSec)}</span>
                        </div>
                        <div class="progress-track">
                            <div class="progress-fill"
                                 style="width:${app.pctOfTop.toFixed(1)}%;background:${color}">
                            </div>
                        </div>
                    </div>`;
            });
        }

        container.innerHTML = heroHTML
            + '<div class="section-label" style="margin-top:16px;">APPS USED</div>'
            + '<div class="app-list">' + appsHTML + '</div>';

    } catch (e) {
        container.innerHTML = '<div class="empty-text">Failed to load data.</div>';
    }
}

function formatDayLabel(isoDate) {
    const d = new Date(isoDate + 'T00:00:00');
    const dayName = d.toLocaleDateString('en-GB', { weekday: 'long' }).toUpperCase();
    const dayNum  = d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
    return `${dayName}, ${dayNum}`; // "MONDAY, 26 May"
}

// Returns "MONDAY, 25 MAY" from "2026-05-25"
function formatDayHeading(isoDate) {
    const d = new Date(isoDate + 'T00:00:00');
    const weekday = d.toLocaleDateString('en-GB', { weekday: 'long' }).toUpperCase();
    const dayMonth = d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
    return `${weekday}, ${dayMonth}`;
}

// Hide window instead of exiting process
window.addEventListener('beforeunload', e => {
  e.preventDefault();
  if (window.hideWindow) {
    window.hideWindow();
  }
});

document.getElementById('clear-data').addEventListener('click', async () => {
    const confirmed = confirm('Delete all usage history? This cannot be undone.');
    if (!confirmed) return;
    const res = await fetch('/api/data', { method: 'DELETE' });
    if (res.ok) {
        alert('All usage data cleared.');
        loadToday();
    } else {
        alert('Failed to clear data. Try again.');
    }
});

document.getElementById('limit-add').addEventListener('click', async () => {
    const appInput = document.getElementById('limit-app');
    const minInput = document.getElementById('limit-min');
    const app     = appInput.value.trim().toLowerCase();
    const minutes = parseInt(minInput.value, 10);
    if (!app || !minutes || minutes <= 0) {
        alert('Enter a valid app name and minutes.');
        return;
    }
    await fetch('/api/limits', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ app, minutes })
    });
    appInput.value = '';
    minInput.value = '';
    loadLimitsList();
});

document.addEventListener('DOMContentLoaded', () => {
    loadSettings();
    loadLimitsList();
    setTimeout(loadToday, 200);
});
