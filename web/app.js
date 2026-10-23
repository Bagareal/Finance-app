import { html, render, useState, useEffect, useMemo, useRef } from './vendor/preact-htm.js';

// i18n.js is loaded with the same ?v=<version> as this file, so a new release is never served stale by a cache.
const { t, tr, configure, locale, language, currencies, money, currencySymbol, toInput, amountPlaceholder, parseCents } =
  await import('./i18n.js' + new URL(import.meta.url).search);

/* ---------- helpers ---------- */

const api = async (method, path, body) => {
  const opt = { method, headers: {}, credentials: 'same-origin' };
  if (body !== undefined) {
    const csv = typeof body === 'string';
    opt.headers['Content-Type'] = csv ? 'text/csv' : 'application/json';
    opt.body = csv ? body : JSON.stringify(body);
  }
  const r = await fetch('/api' + path, opt);
  if (r.status === 401 && !/^\/(login|password|pin|bio|setup)/.test(path)) {
    window.dispatchEvent(new Event('fa-unauth'));
    throw new Error(t('Sign-in required'));
  }
  if (r.status === 204) return null;
  const data = await r.json().catch(() => ({}));
  if (!r.ok) {
    const e = new Error(tr(data.error) || t('Error'));
    e.raw = data.error || ''; // the server's English message, for logic that must not depend on the language
    throw e;
  }
  return data;
};

/** The name of an account as shown in lists and pickers: grouped ones read "Group · Name". */
const accLabel = (a) => (a.group ? a.group + ' · ' + a.name : a.name);
const sign = (kind, c) => (kind === 'income' ? '+' : kind === 'expense' ? '−' : '') + money(c);
const todayStr = () => new Date().toLocaleDateString('sv');
const dayLabel = (d) => new Date(d + 'T00:00').toLocaleDateString(locale(), { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
const shortDate = (d) => new Date(d + 'T00:00').toLocaleDateString(locale(), { day: 'numeric', month: 'short' });
const monthLabel = (y, m) => new Date(y, m - 1, 1).toLocaleDateString(locale(), { month: 'long', year: 'numeric' });
const PALETTE = ['#0f766e', '#2563eb', '#d97706', '#9333ea', '#dc2626', '#0891b2', '#65a30d', '#db2777', '#4f46e5', '#ea580c'];
const KIND = { expense: 'Expense', income: 'Income', transfer: 'Transfer' };
const FREQ = { weekly: 'Every week', monthly: 'Every month', yearly: 'Every year' };
// the stored values are fixed keys (kept from the first version); only the labels are translated
const ACC_TYPES = { corrente: 'Checking', contanti: 'Cash', risparmio: 'Savings', carta: 'Card', investimenti: 'Investments' };
const NEEDS_MOVE = 'choose where to move them';
const TOO_MANY = 'too many attempts';

// Themes are a per-device preference (localStorage); the colours live in style.css under [data-theme].
const THEMES = {
  auto: { label: 'Automatic (system)', colors: null },
  light: { label: 'Light', colors: ['#f4f6f7', '#ffffff', '#0f766e'] },
  dark: { label: 'Dark', colors: ['#0e1417', '#172026', '#2dd4bf'] },
  midnight: { label: 'Midnight', colors: ['#000000', '#0c0c14', '#818cf8'] },
  sand: { label: 'Sand', colors: ['#f5efe4', '#fffaf1', '#b45309'] },
};
const savedTheme = () => { try { return localStorage.getItem('fa_theme') || 'auto'; } catch { return 'auto'; } };
function applyTheme(name) {
  const el = document.documentElement;
  if (!THEMES[name] || name === 'auto') el.removeAttribute('data-theme'); else el.setAttribute('data-theme', name);
  const meta = document.querySelector('meta[name=theme-color]');
  if (meta) meta.setAttribute('content', getComputedStyle(el).getPropertyValue('--surface').trim() || '#0f766e');
}

const toast = (msg) => window.dispatchEvent(new CustomEvent('fa-toast', { detail: msg }));

function useLoad(fn, deps) {
  const [s, setS] = useState({ loading: true });
  useEffect(() => {
    let off = false;
    setS((x) => ({ ...x, loading: true }));
    fn().then((data) => !off && setS({ data }), (e) => !off && setS({ error: e.message }));
    return () => { off = true; };
  }, deps);
  return s;
}

const Icon = ({ d }) => html`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d=${d} /></svg>`;
const ICONS = {
  home: 'M3 11l9-8 9 8M5 10v10h5v-6h4v6h5V10',
  list: 'M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01',
  chart: 'M4 20V10M10 20V4M16 20v-8M22 20H2',
  wallet: 'M3 7h16a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V7zm0 0V6a2 2 0 012-2h12M16 14h2',
  tag: 'M20 12l-8 8-9-9V3h8l9 9zM7.5 7.5h.01',
  repeat: 'M17 2l4 4-4 4M3 11V9a3 3 0 013-3h15M7 22l-4-4 4-4M21 13v2a3 3 0 01-3 3H3',
  gear: 'M12 15a3 3 0 100-6 3 3 0 000 6zM19 12a7 7 0 00-.1-1.2l2-1.5-2-3.4-2.3 1a7 7 0 00-2-1.2L14 3h-4l-.6 2.7a7 7 0 00-2 1.2l-2.3-1-2 3.4 2 1.5A7 7 0 005 12c0 .4 0 .8.1 1.2l-2 1.5 2 3.4 2.3-1a7 7 0 002 1.2L10 21h4l.6-2.7a7 7 0 002-1.2l2.3 1 2-3.4-2-1.5c.1-.4.1-.8.1-1.2z',
  more: 'M5 12h.01M12 12h.01M19 12h.01',
  eye: 'M1 12s4-7 11-7 11 7 11 7-4 7-11 7S1 12 1 12zM12 15a3 3 0 100-6 3 3 0 000 6z',
  pencil: 'M12 20h9M16.5 3.5a2.1 2.1 0 013 3L7 19l-4 1 1-4 12.5-12.5z',
  trash: 'M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6M10 11v6M14 11v6',
  plus: 'M12 5v14M5 12h14',
  eyeoff: 'M17.9 17.9A10.9 10.9 0 0112 19c-7 0-11-7-11-7a19 19 0 014.2-5.2M9.9 4.2A10.7 10.7 0 0112 4c7 0 11 7 11 7a19 19 0 01-3.2 4.2M1 1l22 22M14.1 14.1a3 3 0 01-4.2-4.2',
};

/* ---------- base components ---------- */

function Modal({ title, onClose, children }) {
  useEffect(() => {
    const k = (e) => e.key === 'Escape' && onClose();
    window.addEventListener('keydown', k);
    return () => window.removeEventListener('keydown', k);
  }, []);
  return html`<div class="overlay" onMouseDown=${(e) => e.target === e.currentTarget && onClose()}>
    <div class="modal" role="dialog" aria-modal="true"><h2>${title}</h2>${children}</div></div>`;
}

function Confirm({ title, text, label, onYes, onClose }) {
  const [busy, setBusy] = useState(false);
  return html`<${Modal} title=${title} onClose=${onClose}>
    <p>${text}</p>
    <div class="actions">
      <button class="btn" onClick=${onClose}>${t('Cancel')}</button>
      <button class=${'btn primary' + (label ? '' : ' warn')} disabled=${busy} onClick=${async () => { setBusy(true); await onYes(); }}>${label || t('Delete')}</button>
    </div></${Modal}>`;
}

function MonthNav({ ym, setYm, year }) {
  const step = (d) => {
    if (year) return setYm({ y: ym.y + d, m: 0 });
    let m = ym.m + d, y = ym.y;
    if (m < 1) { m = 12; y--; } else if (m > 12) { m = 1; y++; }
    setYm({ y, m });
  };
  return html`<div class="nav-month">
    <button class="btn small" aria-label=${t('Previous')} onClick=${() => step(-1)}>‹</button>
    <b>${year ? ym.y : monthLabel(ym.y, ym.m)}</b>
    <button class="btn small" aria-label=${t('Next')} onClick=${() => step(1)}>›</button></div>`;
}

const Stats = ({ s }) => html`<div class="stats">
  <div class="stat"><small>${t('Earned')}</small><b class="income num">${money(s.income)}</b></div>
  <div class="stat"><small>${t('Spent')}</small><b class="expense num">${money(s.expense)}</b></div>
  <div class="stat"><small>${t('Saved')}</small><b class=${'num ' + (s.saved < 0 ? 'expense' : '')}>${money(s.saved)}</b></div></div>`;

const EyeBtn = ({ ctx }) => html`<button class="btn small icon" aria-label=${ctx.hide ? t('Show balances') : t('Hide balances')} title=${ctx.hide ? t('Show balances') : t('Hide balances')} onClick=${ctx.toggleHide}><${Icon} d=${ICONS[ctx.hide ? 'eyeoff' : 'eye']} /></button>`;

const RowActions = ({ onEdit, onDelete, onValue, reserve }) => html`<span class="row-actions">
  ${!onValue && reserve && html`<span style="width:36px"></span>`}
  ${onValue && html`<button class="iconbtn" aria-label=${t('Update value')} title=${t('Update value')} onClick=${onValue}><${Icon} d=${ICONS.chart} /></button>`}
  <button class="iconbtn" aria-label=${t('Edit')} title=${t('Edit')} onClick=${onEdit}><${Icon} d=${ICONS.pencil} /></button>
  <button class="iconbtn danger" aria-label=${t('Delete')} title=${t('Delete')} onClick=${onDelete}><${Icon} d=${ICONS.trash} /></button></span>`;

const Status = ({ s }) => s.error ? html`<p class="err">${s.error}</p>` : s.loading && !s.data ? html`<p class="muted">${t('Loading…')}</p>` : null;

/* ---------- biometrics (WebAuthn) ---------- */

const b64u = {
  enc: (buf) => btoa(String.fromCharCode(...new Uint8Array(buf))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, ''),
  dec: (s) => Uint8Array.from(atob(s.replace(/-/g, '+').replace(/_/g, '/')), (c) => c.charCodeAt(0)),
};
const bioSupported = async () => !!(window.isSecureContext && window.PublicKeyCredential &&
  (await PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable().catch(() => false)));

async function bioRegister() {
  const o = await api('POST', '/bio/register/begin');
  const cred = await navigator.credentials.create({ publicKey: {
    challenge: b64u.dec(o.challenge), rp: o.rp, user: { ...o.user, id: b64u.dec(o.user.id) },
    pubKeyCredParams: o.pubKeyCredParams, authenticatorSelection: o.authenticatorSelection,
    attestation: o.attestation, timeout: o.timeout,
    excludeCredentials: o.excludeCredentials.map((c) => ({ ...c, id: b64u.dec(c.id) })),
  } });
  await api('POST', '/bio/register/finish', {
    clientDataJSON: b64u.enc(cred.response.clientDataJSON), attestationObject: b64u.enc(cred.response.attestationObject),
  });
}

async function bioUnlock() {
  const o = await api('POST', '/bio/login/begin');
  const a = await navigator.credentials.get({ publicKey: {
    challenge: b64u.dec(o.challenge), rpId: o.rpId, timeout: o.timeout, userVerification: o.userVerification,
    allowCredentials: o.allowCredentials.map((c) => ({ ...c, id: b64u.dec(c.id) })),
  } });
  await api('POST', '/bio/login/finish', {
    id: b64u.enc(a.rawId), clientDataJSON: b64u.enc(a.response.clientDataJSON),
    authenticatorData: b64u.enc(a.response.authenticatorData), signature: b64u.enc(a.response.signature),
  });
}

/* ---------- recurring choice ---------- */

function ScopeModal({ type, dateChanged, onPick, onClose }) {
  const del = type === 'del';
  const opts = [
    ['one', t('Only this transaction'), ''],
    ['following', t('This and the following'), del ? t('The series stops.') : t('The series will use the new values.')],
    ['all', t('All transactions in the series'), del ? t('Also deletes the series.') : t('Past ones too.')],
  ];
  return html`<${Modal} title=${del ? t('Delete recurring transaction') : t('Edit recurring transaction')} onClose=${onClose}>
    <p style="margin-top:0">${del ? t('This transaction is part of a recurring series. What do you want to delete?') : t('This transaction is part of a recurring series. What do you want to edit?')}</p>
    <div style="display:grid;gap:8px">
      ${opts.map(([k, label, hint]) => html`<button class=${'btn' + (del && k !== 'one' ? ' danger' : '')} style="text-align:left;padding:10px 14px" onClick=${() => onPick(k)}>
        <b>${label}</b>${hint && html`<div class="muted" style="font-size:.85rem">${hint}</div>`}</button>`)}
    </div>
    ${!del && dateChanged && html`<p class="muted" style="font-size:.85rem">${t('The date changes only for this transaction.')}</p>`}
    <div class="actions"><button class="btn" onClick=${onClose}>${t('Cancel')}</button></div></${Modal}>`;
}

/* ---------- transaction / recurring modal ---------- */

function catOptions(categories, kind) {
  const macros = categories.filter((c) => !c.parent_id && c.kind === kind);
  const out = [];
  for (const m of macros) {
    out.push({ id: m.id, label: m.name });
    for (const s of categories.filter((c) => c.parent_id === m.id)) out.push({ id: s.id, label: `${m.name} › ${s.name}` });
  }
  return out;
}

function FlowModal({ ctx, item, kind: kind0, rec, onClose }) {
  const accounts = ctx.accounts.filter((a) => !a.archived || (item && (item.account_id === a.id || item.to_account_id === a.id)));
  const lastAcc = Number(localStorage.getItem('fa_acc'));
  const [f, setF] = useState(() => item ? {
    kind: item.kind, amount: toInput(item.amount_cents), date: rec ? item.start_date : item.date, account_id: item.account_id,
    to_account_id: item.to_account_id || '', category_id: item.category_id || '', note: item.note,
    freq: item.freq || 'monthly', end_date: item.end_date || '', active: item.active !== false,
  } : {
    kind: kind0 || 'expense', amount: '', date: todayStr(), account_id: accounts.find((a) => a.id === lastAcc)?.id || accounts[0]?.id || '',
    to_account_id: '', category_id: '', note: '', freq: 'monthly', end_date: '', active: true,
  });
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);
  const [ask, setAsk] = useState(null); // { type: 'save' | 'del', body }
  const set = (k) => (e) => setF((x) => ({ ...x, [k]: e.target.type === 'checkbox' ? e.target.checked : e.target.value }));
  const opts = useMemo(() => catOptions(ctx.categories, f.kind), [ctx.categories, f.kind]);
  const valid = opts.some((o) => String(o.id) === String(f.category_id));

  if (!accounts.length) {
    return html`<${Modal} title=${t('No account')} onClose=${onClose}><p>${t('Create an account first from the Accounts section.')}</p>
      <div class="actions"><button class="btn primary" onClick=${() => { onClose(); location.hash = '#/accounts'; }}>${t('Go to accounts')}</button></div></${Modal}>`;
  }

  const generated = !!(item && !rec && item.recurring_id);

  const send = async (body, scope) => {
    setBusy(true);
    try {
      const path = rec ? '/recurring' : '/transactions';
      const q = scope ? `?scope=${scope}` : '';
      await (item ? api('PUT', `${path}/${item.id}${q}`, body) : api('POST', path, body));
      localStorage.setItem('fa_acc', f.account_id);
      ctx.bump();
      onClose();
    } catch (e) { setErr(e.message); setBusy(false); setAsk(null); }
  };
  const save = () => {
    const amount = parseCents(f.amount);
    if (!(amount > 0)) return setErr(t('Enter an amount greater than zero'));
    const body = {
      kind: f.kind, amount_cents: amount, account_id: Number(f.account_id), note: f.note,
      to_account_id: f.kind === 'transfer' ? Number(f.to_account_id) || null : null,
      category_id: f.kind !== 'transfer' && valid ? Number(f.category_id) : null,
    };
    if (rec) Object.assign(body, { freq: f.freq, start_date: f.date, end_date: f.end_date, active: f.active });
    else body.date = f.date;
    if (generated) setAsk({ type: 'save', body });
    else send(body);
  };
  const remove = async (scope) => {
    try { await api('DELETE', `/transactions/${item.id}?scope=${scope}`); ctx.bump(); onClose(); }
    catch (e) { setErr(e.message); setAsk(null); }
  };
  const del = () => generated ? setAsk({ type: 'del' }) : ctx.confirm({
    title: rec ? t('Delete the recurring item?') : t('Delete the transaction?'),
    text: rec ? t('Transactions already created are kept.') : t('This cannot be undone.'),
    run: async () => { await api('DELETE', `${rec ? '/recurring' : '/transactions'}/${item.id}`); ctx.bump(); onClose(); },
  });

  const title = item ? (rec ? t('Edit recurring') : t('Edit transaction')) : (rec ? t('New recurring') : t('New transaction'));
  return html`<${Fragment}><${Modal} title=${title} onClose=${onClose}>
    <div class="seg" style="margin-bottom:14px">
      ${Object.entries(KIND).map(([k, v]) => html`<button type="button" class=${f.kind === k ? 'on' : ''} onClick=${() => setF({ ...f, kind: k, category_id: '' })}>${t(v)}</button>`)}
    </div>
    <div class="two">
      <label class="f">${t('Amount ({symbol})', { symbol: currencySymbol() })}<input inputmode="decimal" placeholder=${amountPlaceholder()} value=${f.amount} onInput=${set('amount')} autofocus /></label>
      <label class="f">${rec ? t('Date of the first one') : t('Date')}<input type="date" value=${f.date} onInput=${set('date')} /></label>
    </div>
    <div class=${f.kind === 'transfer' ? 'two' : ''}>
      <label class="f">${f.kind === 'transfer' ? t('From account') : t('Account')}
        <select value=${f.account_id} onChange=${set('account_id')}>${accounts.map((a) => html`<option value=${a.id}>${accLabel(a)}</option>`)}</select></label>
      ${f.kind === 'transfer' && html`<label class="f">${t('To account')}
        <select value=${f.to_account_id} onChange=${set('to_account_id')}><option value="">—</option>
          ${accounts.filter((a) => String(a.id) !== String(f.account_id)).map((a) => html`<option value=${a.id}>${accLabel(a)}</option>`)}</select></label>`}
    </div>
    ${f.kind !== 'transfer' && html`<label class="f">${t('Category')}
      <select value=${valid ? f.category_id : ''} onChange=${set('category_id')}><option value="">${t('No category')}</option>
        ${opts.map((o) => html`<option value=${o.id}>${o.label}</option>`)}</select></label>`}
    <label class="f">${t('Note')}<input value=${f.note} onInput=${set('note')} maxlength="200" /></label>
    ${rec && html`<div class="two">
      <label class="f">${t('Frequency')}<select value=${f.freq} onChange=${set('freq')}>${Object.entries(FREQ).map(([k, v]) => html`<option value=${k}>${t(v)}</option>`)}</select></label>
      <label class="f">${t('End (optional)')}<input type="date" value=${f.end_date} onInput=${set('end_date')} /></label></div>
      <label class="check"><input type="checkbox" checked=${f.active} onChange=${set('active')} /> ${t('Active')}</label>`}
    ${err && html`<p class="err">${err}</p>`}
    <div class="actions">
      ${item && html`<button class="btn danger left" onClick=${del}>${t('Delete')}</button>`}
      <button class="btn" onClick=${onClose}>${t('Cancel')}</button>
      <button class="btn primary" disabled=${busy} onClick=${save}>${t('Save')}</button>
    </div></${Modal}>
    ${ask && html`<${ScopeModal} type=${ask.type} dateChanged=${item && f.date !== item.date}
      onPick=${(scope) => (ask.type === 'save' ? send(ask.body, scope) : remove(scope))} onClose=${() => setAsk(null)} />`}</${Fragment}>`;
}

/* ---------- transaction rows ---------- */

function TxRow({ ctx, tx }) {
  const transfer = tx.kind === 'transfer';
  // grouped accounts read "Group · Name": the title keeps the short names and the full ones go on the second line
  const grouped = transfer && (ctx.accGroup(tx.account_id) || ctx.accGroup(tx.to_account_id));
  const title = transfer ? `${ctx.accShort(tx.account_id)} → ${ctx.accShort(tx.to_account_id)}` : ctx.catLabel(tx.category_id);
  const full = grouped && `${ctx.accName(tx.account_id)} → ${ctx.accName(tx.to_account_id)}`;
  return html`<div class="row">
    <span class="dot" style=${'background:' + (transfer ? 'var(--transfer)' : ctx.catColor(tx.category_id))}></span>
    <div class="grow"><div>${title}${tx.recurring_id && html`<span class="pill">${t('recurring')}</span>`}</div>
      <div class="muted" style="font-size:.85rem">${[!transfer && ctx.accName(tx.account_id), full, tx.note].filter(Boolean).join(' · ') || ' '}</div></div>
    <b class=${'num ' + tx.kind}>${sign(tx.kind, tx.amount_cents)}</b>
    <${RowActions} onEdit=${() => ctx.openTx(tx)} onDelete=${() => ctx.deleteTx(tx)} /></div>`;
}

function TxList({ ctx, list, dates = true }) {
  if (!list.length) return html`<div class="empty">${t('No transactions')}</div>`;
  let last = '';
  return list.map((tx) => {
    const head = dates && tx.date !== last ? html`<div class="day">${dayLabel(tx.date)}</div>` : null;
    last = tx.date;
    return html`<${Fragment}>${head}<${TxRow} ctx=${ctx} tx=${tx} /></${Fragment}>`;
  });
}
const Fragment = ({ children }) => children;

/* ---------- pages ---------- */

/** Accounts that share a group name collapse into one entry with their total; the others stay single. */
function groupAccounts(list) {
  const out = [], byGroup = new Map();
  for (const a of list) {
    if (!a.group) { out.push({ single: a }); continue; }
    const key = a.group.toLowerCase();
    let g = byGroup.get(key);
    if (!g) { g = { key, name: a.group, accounts: [], total: 0 }; byGroup.set(key, g); out.push(g); }
    g.accounts.push(a);
    g.total += a.balance_cents;
  }
  return out;
}

/** The list of accounts: grouped ones show a single line with the total, which opens to show each account. */
function AccountRows({ ctx, list, flat, actions }) {
  const [open, setOpen] = useState(() => { try { return JSON.parse(localStorage.getItem('fa_open') || '{}'); } catch { return {}; } });
  const toggle = (key) => {
    const next = { ...open, [key]: !open[key] };
    setOpen(next);
    try { localStorage.setItem('fa_open', JSON.stringify(next)); } catch { /* ignore */ }
  };
  const info = (a) => (a.manual ? (a.value_date ? t('value of {date}', { date: shortDate(a.value_date) }) : t('value not set yet')) : t(ACC_TYPES[a.type] || a.type));
  const reserve = !!actions && list.some((a) => a.manual); // keeps the amounts aligned when only some rows have the chart button
  const amount = (c) => html`<b class=${'num ' + (!ctx.hide && c < 0 ? 'expense' : '')}>${ctx.bal(c)}</b>`;
  const row = (a, cls, label) => html`<div class=${'row ' + cls}>
    <div class="grow"><div>${label}</div><div class="muted" style="font-size:.85rem">${info(a)}</div></div>${amount(a.balance_cents)}
    ${actions && html`<${RowActions} onEdit=${() => actions.edit(a)} onDelete=${() => actions.remove(a)} onValue=${a.manual ? () => actions.value(a) : null} reserve=${reserve} />`}</div>`;
  if (flat) return list.map((a) => row(a, '', ctx.accName(a.id)));
  return groupAccounts(list).map((g) => {
    if (g.single) return row(g.single, '', g.single.name);
    const n = g.accounts.length;
    return html`<${Fragment}>
      <div class="row click" onClick=${() => toggle(g.key)}>
        <div class="grow"><div>${g.name} ${open[g.key] ? '▾' : '▸'}</div><div class="muted" style="font-size:.85rem">${n === 1 ? t('1 account') : t('{n} accounts', { n })}</div></div>${amount(g.total)}
        ${actions && html`<span class="row-actions" style=${'width:' + (reserve ? 112 : 74) + 'px'} onClick=${(e) => e.stopPropagation()}>
          <button class="iconbtn" aria-label=${t('Add account to “{name}”', { name: g.name })} title=${t('Add account to “{name}”', { name: g.name })} onClick=${() => actions.add(g.name)}><${Icon} d=${ICONS.plus} /></button></span>`}</div>
      ${open[g.key] && g.accounts.map((a) => row(a, 'acc-sub', a.name))}</${Fragment}>`;
  });
}

function AccountsCard({ ctx }) {
  const live = ctx.accounts.filter((a) => !a.archived);
  const total = live.reduce((s, a) => s + a.balance_cents, 0);
  return html`<div class="card"><h2 style="display:flex;align-items:center;gap:8px">${t('Accounts')} <${EyeBtn} ctx=${ctx} />
      <span class="muted num" style="margin-left:auto;font-weight:400">${ctx.bal(total)}</span></h2>
    ${live.length ? html`<${AccountRows} ctx=${ctx} list=${live} />`
      : html`<div class="empty">${t('No accounts yet.')} <a href="#/accounts">${t('Create one')}</a></div>`}</div>`;
}

function Home({ ctx }) {
  const now = new Date();
  const [ym, setYm] = useState({ y: now.getFullYear(), m: now.getMonth() + 1 });
  const sum = useLoad(() => api('GET', `/summary?year=${ym.y}&month=${ym.m}`), [ym, ctx.tick]);
  const recent = useLoad(() => api('GET', '/transactions?limit=8'), [ctx.tick]);
  return html`<div>
    <div class="head"><h1>${t('Home')}</h1><${MonthNav} ym=${ym} setYm=${setYm} /></div>
    <${Status} s=${sum} />
    <div class="cols">
      <div>
        ${sum.data && html`<${Stats} s=${sum.data} />`}
        <div class="card"><h2>${t('Last transactions')}</h2>${recent.data ? html`<${TxList} ctx=${ctx} list=${recent.data} dates=${false} />` : html`<${Status} s=${recent} />`}</div>
        ${sum.data && html`<div class="card desk-only"><h2>${t('Spending by category')}</h2><${CatBreakdown} list=${sum.data.expenses} total=${sum.data.expense} kind="expense" /></div>`}
      </div>
      <div class="desk-only"><${AccountsCard} ctx=${ctx} /></div>
    </div></div>`;
}

function Transactions({ ctx }) {
  const [f, setF] = useState({ month: '', account: '', category: '', kind: '' });
  const [limit, setLimit] = useState(100);
  const set = (k) => (e) => { setF({ ...f, [k]: e.target.value }); setLimit(100); };
  const q = new URLSearchParams({ limit });
  if (f.month) { q.set('from', f.month + '-01'); q.set('to', f.month + '-31'); }
  for (const k of ['account', 'category', 'kind']) if (f[k]) q.set(k, f[k]);
  const res = useLoad(() => api('GET', '/transactions?' + q), [q.toString(), ctx.tick]);
  const macros = ctx.categories.filter((c) => !c.parent_id);
  return html`<div>
    <div class="head"><h1>${t('Transactions')}</h1></div>
    <div class="filters">
      <input type="month" value=${f.month} onInput=${set('month')} aria-label=${t('Month')} />
      <select value=${f.account} onChange=${set('account')}><option value="">${t('All accounts')}</option>${ctx.accounts.map((a) => html`<option value=${a.id}>${accLabel(a)}</option>`)}</select>
      <select value=${f.category} onChange=${set('category')}><option value="">${t('All categories')}</option>${macros.map((c) => html`<option value=${c.id}>${c.name}</option>`)}</select>
      <select value=${f.kind} onChange=${set('kind')}><option value="">${t('All types')}</option>${Object.entries(KIND).map(([k, v]) => html`<option value=${k}>${t(v)}</option>`)}</select></div>
    <${Status} s=${res} />
    ${res.data && html`<div class="card"><${TxList} ctx=${ctx} list=${res.data} />
      ${res.data.length >= limit && html`<div style="text-align:center;padding-top:10px"><button class="btn" onClick=${() => setLimit(limit + 100)}>${t('Show more')}</button></div>`}</div>`}</div>`;
}

function Donut({ items, total }) {
  let acc = 0;
  return html`<svg viewBox="0 0 42 42" role="img" aria-label=${t('Spending by category')}>
    <circle cx="21" cy="21" r="15.9155" fill="none" stroke="var(--line)" stroke-width="6" />
    ${items.map((it) => {
      const pct = (it.cents / total) * 100, off = 25 - acc;
      acc += pct;
      return html`<circle cx="21" cy="21" r="15.9155" fill="none" stroke=${it.color} stroke-width="6" stroke-dasharray=${`${pct} ${100 - pct}`} stroke-dashoffset=${off} />`;
    })}
    <text x="21" y="22.5" text-anchor="middle" font-size="4.2" fill="var(--text)" font-weight="600">${money(total)}</text></svg>`;
}

function CatBreakdown({ list, total, kind }) {
  const [open, setOpen] = useState({});
  const items = list.map((c, i) => ({ ...c, color: c.color || PALETTE[i % PALETTE.length], name: c.category_id ? c.name : t('No category') }));
  if (!items.length) return html`<div class="empty">${t('No data')}</div>`;
  return html`<div class="donut-wrap">
    ${kind === 'expense' && html`<${Donut} items=${items} total=${total} />`}
    <div class="legend">${items.map((c) => html`<div>
      <div class="row click" onClick=${() => c.subs?.length && setOpen({ ...open, [c.category_id]: !open[c.category_id] })}>
        <span class="dot" style=${'background:' + c.color}></span>
        <div class="grow"><div>${c.name}${c.subs?.length ? (open[c.category_id] ? ' ▾' : ' ▸') : ''}</div></div>
        <span class="muted num">${Math.round((c.cents / total) * 100)}%</span><b class="num">${money(c.cents)}</b></div>
      ${open[c.category_id] && c.subs.map((s) => html`<div class="row subrow"><div class="grow"><div>${s.name}</div></div><span class="num">${money(s.cents)}</span></div>`)}</div>`)}</div></div>`;
}

function Bars({ months }) {
  const max = Math.max(1, ...months.flatMap((m) => [m.income, m.expense]));
  const W = 360, H = 190, base = 165, top = 10, gw = W / 12;
  const h = (v) => ((base - top) * v) / max;
  const letter = (i) => new Date(2000, i, 1).toLocaleDateString(locale(), { month: 'narrow' });
  return html`<svg class="bars" viewBox=${`0 0 ${W} ${H}`} role="img" aria-label=${t('Income and spending by month')}>
    <line x1="0" x2=${W} y1=${base} y2=${base} stroke="var(--line)" />
    ${months.map((m, i) => html`<g>
      <rect x=${i * gw + 5} y=${base - h(m.income)} width="9" height=${h(m.income)} rx="2" fill="var(--income)" />
      <rect x=${i * gw + 15} y=${base - h(m.expense)} width="9" height=${h(m.expense)} rx="2" fill="var(--expense)" />
      <text x=${i * gw + gw / 2} y=${base + 14} text-anchor="middle">${letter(i)}</text></g>`)}</svg>`;
}

function Summary({ ctx }) {
  const now = new Date();
  const [year, setYear] = useState(false);
  const [ym, setYm] = useState({ y: now.getFullYear(), m: now.getMonth() + 1 });
  const cur = year ? { y: ym.y, m: 0 } : ym;
  const res = useLoad(() => api('GET', `/summary?year=${cur.y}&month=${cur.m}`), [cur.y, cur.m, ctx.tick]);
  const toggle = (y) => { setYear(y); if (!y && !ym.m) setYm({ y: ym.y, m: now.getMonth() + 1 }); };
  const s = res.data;
  return html`<div>
    <div class="head"><h1>${t('Summary')}</h1>
      <div class="seg"><button class=${!year ? 'on' : ''} onClick=${() => toggle(false)}>${t('Month')}</button><button class=${year ? 'on' : ''} onClick=${() => toggle(true)}>${t('Year')}</button></div></div>
    <div class="head"><${MonthNav} ym=${cur} setYm=${setYm} year=${year} /></div>
    <${Status} s=${res} />
    ${s && html`<${Stats} s=${s} />
      <div class="cols2">
        <div class="card"><h2>${t('Spending by category')}</h2><${CatBreakdown} list=${s.expenses} total=${s.expense} kind="expense" /></div>
        <div class="card"><h2>${t('Income by category')}</h2><${CatBreakdown} list=${s.incomes} total=${s.income} kind="income" /></div></div>
      <div class="card"><h2>${t('Trend {year}', { year: cur.y })}</h2><${Bars} months=${s.months} />
        <div class="muted" style="font-size:.8rem;text-align:center"><span class="income">■</span> ${t('income')}   <span class="expense">■</span> ${t('spending')}</div></div>`}</div>`;
}

function AccountModal({ ctx, item, preset, onClose }) {
  const [f, setF] = useState(item
    ? { name: item.name, type: item.type, initial: toInput(item.initial_cents), archived: item.archived, group: item.group, manual: item.manual }
    : { name: '', type: 'corrente', initial: '', archived: false, group: preset?.group || '', manual: false });
  const [err, setErr] = useState('');
  const set = (k) => (e) => setF({ ...f, [k]: e.target.type === 'checkbox' ? e.target.checked : e.target.value });
  const groups = [...new Set(ctx.accounts.map((a) => a.group).filter(Boolean))];
  const save = async () => {
    const init = f.initial.trim() === '' ? 0 : parseCents(f.initial);
    if (Number.isNaN(init)) return setErr(t('Invalid opening balance'));
    const body = { name: f.name, type: f.type, initial_cents: init, archived: f.archived, group: f.group, manual: f.manual };
    try { await (item ? api('PUT', `/accounts/${item.id}`, body) : api('POST', '/accounts', body)); ctx.bump(); onClose(); } catch (e) { setErr(e.message); }
  };
  const del = () => ctx.confirm({
    title: t('Delete “{name}”?', { name: item.name }), text: t('Only an account without transactions can be deleted; otherwise archive it.'),
    run: async () => { await api('DELETE', `/accounts/${item.id}`); ctx.bump(); onClose(); },
  });
  return html`<${Modal} title=${item ? t('Edit account') : t('New account')} onClose=${onClose}>
    <label class="f">${t('Name')}<input value=${f.name} onInput=${set('name')} autofocus /></label>
    <div class="two">
      <label class="f">${t('Type')}<select value=${f.type} onChange=${set('type')}>${Object.entries(ACC_TYPES).map(([k, v]) => html`<option value=${k}>${t(v)}</option>`)}</select></label>
      <label class="f">${t('Opening balance ({symbol})', { symbol: currencySymbol() })}<input inputmode="decimal" placeholder=${amountPlaceholder()} value=${f.initial} onInput=${set('initial')} /></label></div>
    <label class="f">${t('Group (optional)')}<input list="acc-groups" value=${f.group} onInput=${set('group')} placeholder=${t('e.g. a bank or a broker')} /></label>
    <datalist id="acc-groups">${groups.map((g) => html`<option value=${g}></option>`)}</datalist>
    <label class="check"><input type="checkbox" checked=${f.manual} onChange=${set('manual')} /> ${t('Value updated by hand (e.g. investments)')}</label>
    ${f.manual && html`<p class="muted" style="margin:-4px 0 12px;font-size:.85rem">${t('Transfers made after the last update are added to the value.')}</p>`}
    ${item && html`<label class="check"><input type="checkbox" checked=${f.archived} onChange=${set('archived')} /> ${t('Archived (hidden from new transactions)')}</label>`}
    ${err && html`<p class="err">${err}</p>`}
    <div class="actions">${item && html`<button class="btn danger left" onClick=${del}>${t('Delete')}</button>`}
      <button class="btn" onClick=${onClose}>${t('Cancel')}</button><button class="btn primary" onClick=${save}>${t('Save')}</button></div></${Modal}>`;
}

function Sparkline({ points }) {
  const W = 300, H = 70, pad = 6;
  const vals = points.map((p) => p.value_cents);
  const min = Math.min(...vals), max = Math.max(...vals), span = max - min || 1;
  const x = (i) => pad + (points.length === 1 ? (W - 2 * pad) / 2 : (i * (W - 2 * pad)) / (points.length - 1));
  const y = (v) => H - pad - ((v - min) / span) * (H - 2 * pad);
  const path = points.map((p, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)} ${y(p.value_cents).toFixed(1)}`).join(' ');
  return html`<svg viewBox=${`0 0 ${W} ${H}`} role="img" aria-label=${t('History')} style="width:100%;height:auto;max-height:90px;display:block;margin:4px 0 10px">
    <path d=${path} fill="none" stroke="var(--primary)" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />
    ${points.map((p, i) => html`<circle cx=${x(i)} cy=${y(p.value_cents)} r="3" fill="var(--primary)" />`)}</svg>`;
}

function ValueModal({ ctx, acc, onClose }) {
  const [n, setN] = useState(0);
  const hist = useLoad(() => api('GET', `/accounts/${acc.id}/values`), [acc.id, n]);
  const [date, setDate] = useState(todayStr());
  const [value, setValue] = useState(toInput(acc.balance_cents));
  const [err, setErr] = useState('');
  const save = async () => {
    const cents = parseCents(value);
    if (value.trim() === '' || Number.isNaN(cents)) return setErr(t('Invalid value'));
    try { await api('POST', `/accounts/${acc.id}/values`, { date, value_cents: cents }); ctx.bump(); onClose(); } catch (e) { setErr(e.message); }
  };
  const drop = (v) => ctx.confirm({
    title: t('Delete this value?'), text: t('The balance goes back to the previous value.'),
    run: async () => { await api('DELETE', `/account-values/${v.id}`); ctx.bump(); setN((x) => x + 1); },
  });
  const list = hist.data || [];
  return html`<${Modal} title=${t('Update value: {name}', { name: ctx.accName(acc.id) })} onClose=${onClose}>
    <div class="two">
      <label class="f">${t('Value on')}<input type="date" value=${date} onInput=${(e) => setDate(e.target.value)} /></label>
      <label class="f">${t('Value ({symbol})', { symbol: currencySymbol() })}<input inputmode="decimal" placeholder=${amountPlaceholder()} value=${value} onInput=${(e) => setValue(e.target.value)} autofocus /></label></div>
    ${err && html`<p class="err">${err}</p>`}
    <h3 style="margin:14px 0 4px;font-size:.95rem">${t('History')}</h3>
    ${list.length > 1 && html`<${Sparkline} points=${[...list].reverse()} />`}
    ${hist.data && !list.length && html`<div class="empty">${t('No values yet.')}</div>`}
    <div style="max-height:200px;overflow:auto">${list.map((v) => html`<div class="row">
      <div class="grow"><div>${new Date(v.date + 'T00:00').toLocaleDateString(locale(), { day: 'numeric', month: 'short', year: 'numeric' })}</div>
        <div class=${'num ' + (v.change_cents < 0 ? 'expense' : 'income')} style="font-size:.85rem" title=${t('gain/loss')}>${v.change_cents > 0 ? '+' : v.change_cents < 0 ? '−' : ''}${money(Math.abs(v.change_cents))}</div></div>
      <b class="num">${money(v.value_cents)}</b>
      <button class="iconbtn danger" aria-label=${t('Delete')} title=${t('Delete')} onClick=${() => drop(v)}><${Icon} d=${ICONS.trash} /></button></div>`)}</div>
    <div class="actions"><button class="btn" onClick=${onClose}>${t('Cancel')}</button><button class="btn primary" onClick=${save}>${t('Save')}</button></div></${Modal}>`;
}

function Accounts({ ctx }) {
  const [edit, setEdit] = useState(null);
  const [valueFor, setValueFor] = useState(null);
  const live = ctx.accounts.filter((a) => !a.archived), old = ctx.accounts.filter((a) => a.archived);
  const total = live.reduce((s, a) => s + a.balance_cents, 0);
  const remove = (a) => ctx.confirm({
    title: t('Delete “{name}”?', { name: a.name }), text: t('Only an account without transactions can be deleted; otherwise archive it.'),
    run: async () => { await api('DELETE', `/accounts/${a.id}`); ctx.bump(); },
  });
  const actions = { edit: setEdit, remove, value: setValueFor, add: (group) => setEdit({ group }) };
  const rows = (l, flat) => html`<${AccountRows} ctx=${ctx} list=${l} flat=${flat} actions=${actions} />`;
  return html`<div>
    <div class="head"><h1>${t('Accounts')}</h1><div style="display:flex;gap:8px">
      <button class="btn" onClick=${() => ctx.openTx(null, 'transfer')} disabled=${live.length < 2}>${t('Transfer')}</button>
      <button class="btn primary" onClick=${() => setEdit({})}>${t('New account')}</button></div></div>
    <div class="card"><h2 style="display:flex;align-items:center;gap:8px">${t('Total')} <${EyeBtn} ctx=${ctx} /><span class="num" style="margin-left:auto">${ctx.bal(total)}</span></h2>
      ${live.length ? rows(live) : html`<div class="empty">${t('Create your first account')}</div>`}</div>
    ${old.length > 0 && html`<div class="card"><h2 class="muted">${t('Archived')}</h2>${rows(old, true)}</div>`}
    ${edit && html`<${AccountModal} ctx=${ctx} item=${edit.id ? edit : null} preset=${edit.id ? null : edit} onClose=${() => setEdit(null)} />`}
    ${valueFor && html`<${ValueModal} ctx=${ctx} acc=${valueFor} onClose=${() => setValueFor(null)} />`}</div>`;
}

function CatModal({ ctx, state, onClose }) {
  const { item, parent, kind } = state;
  const [name, setName] = useState(item?.name || '');
  const [err, setErr] = useState('');
  const save = async () => {
    try {
      if (item) await api('PUT', `/categories/${item.id}`, { name, color: item.color });
      else {
        const n = ctx.categories.filter((c) => !c.parent_id).length;
        await api('POST', '/categories', { name, kind, parent_id: parent?.id || null, color: parent ? '' : PALETTE[n % PALETTE.length] });
      }
      ctx.bump(); onClose();
    } catch (e) { setErr(e.message); }
  };
  const title = item ? t('Rename') : parent ? t('New subcategory of {name}', { name: parent.name }) : kind === 'expense' ? t('New expense category') : t('New income category');
  return html`<${Modal} title=${title} onClose=${onClose}>
    <label class="f">${t('Name')}<input value=${name} onInput=${(e) => setName(e.target.value)} onKeyDown=${(e) => e.key === 'Enter' && save()} autofocus /></label>
    ${err && html`<p class="err">${err}</p>`}
    <div class="actions"><button class="btn" onClick=${onClose}>${t('Cancel')}</button><button class="btn primary" onClick=${save}>${t('Save')}</button></div></${Modal}>`;
}

function MoveModal({ ctx, cat, onClose }) {
  const tree = new Set([cat.id, ...ctx.categories.filter((c) => c.parent_id === cat.id).map((c) => c.id)]);
  const opts = catOptions(ctx.categories, cat.kind).filter((o) => !tree.has(o.id));
  const [to, setTo] = useState(opts[0]?.id || '');
  const [err, setErr] = useState('');
  const go = async () => {
    try { await api('DELETE', `/categories/${cat.id}?move_to=${to}`); ctx.bump(); onClose(); } catch (e) { setErr(e.message); }
  };
  return html`<${Modal} title=${t('Delete “{name}”?', { name: cat.name })} onClose=${onClose}>
    <p>${t('There are transactions or recurring items in this category. Choose where to move them.')}</p>
    ${opts.length ? html`<label class="f">${t('Move to')}<select value=${to} onChange=${(e) => setTo(e.target.value)}>${opts.map((o) => html`<option value=${o.id}>${o.label}</option>`)}</select></label>`
      : html`<p class="err">${t('Create another category of the same type first.')}</p>`}
    ${err && html`<p class="err">${err}</p>`}
    <div class="actions"><button class="btn" onClick=${onClose}>${t('Cancel')}</button><button class="btn primary" disabled=${!opts.length} onClick=${go}>${t('Move and delete')}</button></div></${Modal}>`;
}

function Categories({ ctx }) {
  const [kind, setKind] = useState('expense');
  const [modal, setModal] = useState(null);
  const [open, setOpen] = useState({});
  const macros = ctx.categories.filter((c) => !c.parent_id && c.kind === kind);
  const count = (k) => ctx.categories.filter((c) => !c.parent_id && c.kind === k).length;
  const remove = (c) => ctx.confirm({
    title: t('Delete “{name}”?', { name: c.name }), text: c.parent_id ? '' : t('Its subcategories will be deleted too.'),
    run: async () => {
      try { await api('DELETE', `/categories/${c.id}`); ctx.bump(); }
      catch (e) { if ((e.raw || '').includes(NEEDS_MOVE)) setModal({ move: c }); else toast(e.message); }
    },
  });
  const btns = (c, withSub) => html`<span style="display:flex;gap:4px" onClick=${(e) => e.stopPropagation()}>
    ${withSub && html`<button class="btn small" onClick=${() => { setOpen({ ...open, [c.id]: true }); setModal({ parent: c, kind }); }}>${t('+ Sub')}</button>`}
    <button class="btn small" onClick=${() => setModal({ item: c, kind })}>${t('Edit')}</button>
    <button class="btn small danger" aria-label=${t('Delete')} onClick=${() => remove(c)}>✕</button></span>`;
  return html`<div>
    <div class="head"><h1>${t('Categories')}</h1></div>
    <div class="seg full" style="margin-bottom:14px">
      <button class=${kind === 'expense' ? 'on' : ''} onClick=${() => setKind('expense')}>${t('Expenses')} (${count('expense')})</button>
      <button class=${kind === 'income' ? 'on' : ''} onClick=${() => setKind('income')}>${t('Income|plural')} (${count('income')})</button></div>
    <div class="card">
      ${macros.length ? macros.map((m, i) => {
        const subs = ctx.categories.filter((c) => c.parent_id === m.id);
        return html`<div>
          <div class=${'row' + (subs.length ? ' click' : '')} onClick=${() => subs.length && setOpen({ ...open, [m.id]: !open[m.id] })}>
            <span class="dot" style=${'background:' + (m.color || PALETTE[i % PALETTE.length])}></span>
            <div class="grow"><div><b>${m.name}</b>${subs.length ? html` <span class="muted">${open[m.id] ? '▾' : '▸'} ${subs.length}</span>` : ''}</div></div>${btns(m, true)}</div>
          ${open[m.id] && subs.map((sc) => html`<div class="row subrow"><div class="grow"><div>${sc.name}</div></div>${btns(sc, false)}</div>`)}</div>`;
      }) : html`<div class="empty">${kind === 'expense' ? t('No expense categories') : t('No income categories')}</div>`}
      <div style="padding-top:10px"><button class="btn primary" onClick=${() => setModal({ kind })}>${t('New category')}</button></div></div>
    ${modal?.move ? html`<${MoveModal} ctx=${ctx} cat=${modal.move} onClose=${() => setModal(null)} />`
      : modal && html`<${CatModal} ctx=${ctx} state=${modal} onClose=${() => setModal(null)} />`}</div>`;
}

function Recurring({ ctx }) {
  const res = useLoad(() => api('GET', '/recurring'), [ctx.tick]);
  const list = res.data || [];
  const toggle = async (r) => { try { await api('PUT', `/recurring/${r.id}`, { ...r, active: !r.active }); ctx.bump(); } catch (e) { toast(e.message); } };
  return html`<div>
    <div class="head"><h1>${t('Recurring')}</h1><button class="btn primary" onClick=${() => ctx.openRec(null)}>${t('New')}</button></div>
    <${Status} s=${res} />
    <div class="card">${list.length ? list.map((r) => html`<div class="row">
      <div class="grow">
        <div>${r.kind === 'transfer' ? `${ctx.accName(r.account_id)} → ${ctx.accName(r.to_account_id)}` : ctx.catLabel(r.category_id)}${!r.active && html`<span class="pill">${t('paused')}</span>`}</div>
        <div class="muted" style="font-size:.85rem">${t(FREQ[r.freq])} · ${r.active ? t('next {date}', { date: shortDate(r.next_date) }) : t('stopped')}${r.note ? ' · ' + r.note : ''}</div></div>
      <b class=${'num ' + r.kind}>${sign(r.kind, r.amount_cents)}</b>
      <button class="btn small" onClick=${() => toggle(r)}>${r.active ? t('Pause') : t('Resume')}</button>
      <${RowActions} onEdit=${() => ctx.openRec(r)} onDelete=${() => ctx.deleteRec(r)} /></div>`)
      : !res.loading && html`<div class="empty">${t('No recurring items. Add rent, salary, subscriptions…')}</div>`}</div></div>`;
}

function QuickUnlock({ st, reload }) {
  const [pw, setPw] = useState('');
  const [pin, setPin] = useState('');
  const [bio, setBio] = useState(false);
  useEffect(() => { bioSupported().then(setBio); }, []);
  const run = async (fn, ok) => {
    try { await fn(); toast(ok); setPw(''); setPin(''); reload(); } catch (e) { if (e.name !== 'NotAllowedError') toast(e.message || t('Operation failed')); }
  };
  const lockNow = async () => { sessionStorage.removeItem('fa_unlocked'); await api('POST', '/logout'); reload(); };
  return html`<div class="card"><h2>${t('Quick unlock')}</h2>
    <p class="muted" style="margin-top:0">${bio
      ? t('After signing in with the password you can unlock with a PIN or biometrics. The app locks itself after 5 minutes in the background.')
      : t('After signing in with the password you can unlock with a PIN. The app locks itself after 5 minutes in the background.')}</p>
    <label class="f">${t('Current password')}<input type="password" autocomplete="current-password" value=${pw} onInput=${(e) => setPw(e.target.value)} /></label>
    <label class="f">${st.pin ? t('New PIN (4-6 digits)') : t('PIN (4-6 digits)')}<input type="password" inputmode="numeric" pattern="[0-9]*" maxlength="6" autocomplete="off" value=${pin} onInput=${(e) => setPin(e.target.value.replace(/\D/g, ''))} /></label>
    <div style="display:flex;gap:8px;flex-wrap:wrap;margin-bottom:6px">
      <button class="btn primary" onClick=${() => run(() => api('POST', '/pin/set', { password: pw, pin }), st.pin ? t('PIN updated') : t('PIN turned on'))}>${st.pin ? t('Change PIN') : t('Turn on PIN')}</button>
      ${st.pin && html`<button class="btn danger" onClick=${() => run(() => api('POST', '/pin/remove', { password: pw }), t('PIN turned off'))}>${t('Turn off PIN')}</button>`}</div>
    <div style="border-top:1px solid var(--line);margin-top:14px;padding-top:14px">
      ${bio ? (st.bio
        ? html`<p style="margin-top:0">${t('Biometrics are on for this device.')}</p><button class="btn danger" onClick=${() => run(() => api('POST', '/bio/remove'), t('Biometrics turned off'))}>${t('Turn off biometrics')}</button>`
        : html`<button class="btn" onClick=${() => run(bioRegister, t('Biometrics turned on'))}>${t('Turn on fingerprint / Face ID on this device')}</button>`)
        : html`<p class="muted" style="margin:0">${window.isSecureContext ? t('Biometrics are not available on this device.') : t('Biometrics need HTTPS: open the app from an https:// address (reverse proxy, Tailscale, domain).')}</p>`}
      ${(st.pin || st.bio) && html`<div style="margin-top:14px"><button class="btn" onClick=${lockNow}>${t('Lock now')}</button></div>`}</div></div>`;
}

function Appearance() {
  const [theme, setTheme] = useState(savedTheme);
  const pick = (name) => {
    try { localStorage.setItem('fa_theme', name); } catch { /* private mode: the theme just won't persist */ }
    applyTheme(name);
    setTheme(name);
  };
  return html`<div class="card"><h2>${t('Appearance')}</h2>
    <div class="themes" role="radiogroup" aria-label=${t('Theme')}>
      ${Object.entries(THEMES).map(([k, v]) => html`<button type="button" role="radio" aria-checked=${theme === k} class=${'theme' + (theme === k ? ' on' : '')} onClick=${() => pick(k)}>
        <span class="swatch" style=${v.colors ? 'background:' + v.colors[0] : 'background:linear-gradient(135deg,#f4f6f7 50%,#0e1417 50%)'}>
          ${v.colors && html`<i style=${'background:' + v.colors[1]}></i><b style=${'background:' + v.colors[2]}></b>`}</span>
        <span>${t(v.label)}</span></button>`)}
    </div>
    <p class="muted" style="margin:10px 0 0;font-size:.85rem">${t('Saved on this device only.')}</p></div>`;
}

function Preferences({ st, reload }) {
  const [err, setErr] = useState('');
  const change = async (patch) => {
    setErr('');
    try { await api('POST', '/settings', patch); reload(); } catch (e) { setErr(e.message); }
  };
  return html`<div class="card"><h2>${t('Language and currency')}</h2>
    <label class="f">${t('Language')}<select value=${st.lang} onChange=${(e) => change({ language: e.target.value })}>
      <option value="en">English</option><option value="it">Italiano</option></select></label>
    <label class="f">${t('Currency')}<select value=${st.currency} onChange=${(e) => change({ currency: e.target.value })}>
      ${currencies().map(([code, name]) => html`<option value=${code}>${code} · ${t(name)}</option>`)}</select></label>
    <p class="muted" style="margin:0;font-size:.85rem">${t('Changing the currency only changes the symbol: amounts are not converted.')}</p>
    ${err && html`<p class="err">${err}</p>`}</div>`;
}

function Settings({ ctx, st, reload, onLogout }) {
  const [pw, setPw] = useState({ current: '', next: '' });
  const [msg, setMsg] = useState('');
  const change = async () => {
    try { await api('POST', '/password', { current: pw.current, new: pw.next }); setPw({ current: '', next: '' }); setMsg(t('Password updated. PIN and biometrics must be set up again.')); reload(); }
    catch (e) { setMsg(e.message); }
  };
  const importCsv = async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    try { const r = await api('POST', '/import', await file.text()); toast(t('Imported {n} transactions', { n: r.imported })); ctx.bump(); }
    catch (er) { toast(er.message); }
    e.target.value = '';
  };
  return html`<div>
    <div class="head"><h1>${t('Settings')}</h1></div>
    <div class="cols2">
      <div>
        <${Appearance} />
        <${Preferences} st=${st} reload=${reload} />
        <${QuickUnlock} st=${st} reload=${reload} />
        <div class="card"><h2>${t('Change password')}</h2>
          <label class="f">${t('Current password')}<input type="password" autocomplete="current-password" value=${pw.current} onInput=${(e) => setPw({ ...pw, current: e.target.value })} /></label>
          <label class="f">${t('New password (min. 6 characters)')}<input type="password" autocomplete="new-password" value=${pw.next} onInput=${(e) => setPw({ ...pw, next: e.target.value })} /></label>
          ${msg && html`<p class="muted">${msg}</p>`}
          <button class="btn primary" onClick=${change}>${t('Update')}</button></div>
      </div>
      <div>
        <div class="card"><h2>${t('Data')}</h2>
          <p class="muted" style="margin-top:0">${t('The CSV has the columns: date, type, amount, account, destination_account, category, subcategory, note. Missing accounts and categories are created.')}</p>
          <div style="display:flex;gap:8px;flex-wrap:wrap">
            <a class="btn" href=${'/api/export.csv?lang=' + language()} download>${t('Export CSV')}</a>
            <label class="btn" style="cursor:pointer">${t('Import CSV')}<input type="file" accept=".csv,text/csv" hidden onChange=${importCsv} /></label>
            <a class="btn" href="/api/backup" download>${t('Download backup')}</a></div></div>
        <div class="card"><button class="btn" onClick=${onLogout}>${t('Sign out')}</button> <span class="muted" style="margin-left:8px">${t('Version {version}', { version: st.version })}</span></div>
      </div>
    </div></div>`;
}

function More() {
  const items = [['categories', 'tag', 'Categories'], ['recurring', 'repeat', 'Recurring'], ['settings', 'gear', 'Settings']];
  return html`<div><div class="head"><h1>${t('More')}</h1></div><div class="card">
    ${items.map(([r, i, l]) => html`<a class="row" href=${'#/' + r} style="text-decoration:none;color:inherit"><${Icon} d=${ICONS[i]} /><div class="grow">${t(l)}</div><span class="muted">›</span></a>`)}</div></div>`;
}

/* ---------- sign-in ---------- */

function Auth({ setup, onDone, onBack }) {
  const [pw, setPw] = useState('');
  const [pw2, setPw2] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);
  const go = async (e) => {
    e.preventDefault();
    if (setup && pw !== pw2) return setErr(t('The passwords do not match'));
    setBusy(true);
    try { await api('POST', setup ? '/setup' : '/login', { password: pw }); sessionStorage.setItem('fa_unlocked', '1'); onDone(); } catch (er) { setErr(er.message); setBusy(false); }
  };
  return html`<form class="auth" onSubmit=${go}>
    <h1>Finance App</h1>
    <p class="muted">${setup ? t('Choose a password to protect your data.') : t('Enter the password.')}</p>
    <label class="f">${t('Password')}<input type="password" autocomplete=${setup ? 'new-password' : 'current-password'} value=${pw} onInput=${(e) => setPw(e.target.value)} autofocus /></label>
    ${setup && html`<label class="f">${t('Repeat the password')}<input type="password" autocomplete="new-password" value=${pw2} onInput=${(e) => setPw2(e.target.value)} /></label>`}
    ${err && html`<p class="err">${err}</p>`}
    <button class="btn primary" style="width:100%" disabled=${busy}>${setup ? t('Get started') : t('Sign in')}</button>
    ${onBack && html`<p style="margin-top:14px"><a href="#" onClick=${(e) => { e.preventDefault(); onBack(); }}>${t('← Back to PIN')}</a></p>`}</form>`;
}

function Lock({ st, onDone, onPassword }) {
  const [pin, setPin] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);
  const len = st.pin_len || 4;
  const bioOk = st.bio && window.isSecureContext;
  const unlocked = () => { sessionStorage.setItem('fa_unlocked', '1'); onDone(); };
  const tryBio = async () => {
    try { await bioUnlock(); unlocked(); } catch (e) { if (e.name !== 'NotAllowedError' && e.name !== 'AbortError') setErr(e.message || t('Biometric unlock failed')); }
  };
  useEffect(() => { if (bioOk) tryBio(); }, []);
  const submit = async (value) => {
    setBusy(true);
    try { await api('POST', '/pin/login', { pin: value }); unlocked(); }
    catch (e) { setErr(e.message); setPin(''); setBusy(false); if ((e.raw || '').includes(TOO_MANY)) setTimeout(onDone, 1200); }
  };
  const press = (d) => {
    if (busy) return;
    setErr('');
    const next = (pin + d).slice(0, len);
    setPin(next);
    if (next.length === len) submit(next);
  };
  useEffect(() => {
    const k = (e) => { if (/^\d$/.test(e.key)) press(e.key); else if (e.key === 'Backspace') setPin((p) => p.slice(0, -1)); };
    window.addEventListener('keydown', k);
    return () => window.removeEventListener('keydown', k);
  });
  return html`<div class="auth lock">
    <h1>Finance App</h1>
    <p class="muted">${st.pin ? t('Enter your PIN') : t('Unlock with biometrics')}</p>
    ${st.pin && html`<div class="dots" aria-label="PIN">${Array.from({ length: len }, (_, i) => html`<span class=${i < pin.length ? 'on' : ''}></span>`)}</div>`}
    ${err && html`<p class="err" style="text-align:center">${err}</p>`}
    ${st.pin && html`<div class="keypad">
      ${[1, 2, 3, 4, 5, 6, 7, 8, 9].map((d) => html`<button type="button" onClick=${() => press(String(d))}>${d}</button>`)}
      ${bioOk ? html`<button type="button" aria-label=${t('Use biometrics')} onClick=${tryBio}>☝</button>` : html`<span></span>`}
      <button type="button" onClick=${() => press('0')}>0</button>
      <button type="button" aria-label=${t('Backspace')} onClick=${() => setPin(pin.slice(0, -1))}>⌫</button></div>`}
    ${!st.pin && bioOk && html`<button class="btn primary" style="width:100%" onClick=${tryBio}>${t('Unlock')}</button>`}
    <p style="text-align:center;margin-top:18px"><a href="#" onClick=${(e) => { e.preventDefault(); onPassword(); }}>${t('Use the password')}</a></p></div>`;
}

/* ---------- app ---------- */

const TABS = [
  ['home', 'home', 'Home'], ['transactions', 'list', 'Transactions'], ['summary', 'chart', 'Summary'], ['accounts', 'wallet', 'Accounts'],
  ['categories', 'tag', 'Categories', 'd'], ['recurring', 'repeat', 'Recurring', 'd'], ['settings', 'gear', 'Settings', 'd'], ['more', 'more', 'More', 'm'],
];
const PAGES = { home: Home, transactions: Transactions, summary: Summary, accounts: Accounts, categories: Categories, recurring: Recurring, more: More };
const WIDE = ['home', 'summary'];
// addresses used by the first versions (Italian), still open in browsers and PWAs after an update
const LEGACY_ROUTES = { movimenti: 'transactions', riepilogo: 'summary', conti: 'accounts', categorie: 'categories', ricorrenti: 'recurring', impostazioni: 'settings', altro: 'more' };
const KNOWN_PAGES = [...Object.keys(PAGES), 'settings'];
/** Reads the page from the address bar and rewrites the address if it was a legacy or unknown one. */
function normalizeHash() {
  const page = pageOf(location.hash.slice(2) || 'home');
  if (location.hash !== '#/' + page) history.replaceState(null, '', '#/' + page);
  return page;
}

/** The page for an address: legacy addresses are translated, unknown ones fall back to Home. */
function pageOf(route) {
  const key = route.split('/')[0];
  const page = LEGACY_ROUTES[key] || key;
  return KNOWN_PAGES.includes(page) ? page : 'home';
}

function Shell({ st, reload, onLogout }) {
  const [route, setRoute] = useState(normalizeHash);
  const [data, setData] = useState({ accounts: [], categories: [], loaded: false });
  const [tick, setTick] = useState(0);
  const [modal, setModal] = useState(null);
  const [conf, setConf] = useState(null);
  const [msg, setMsg] = useState('');
  const [delScope, setDelScope] = useState(null); // a transaction from a recurring series waiting for the scope choice
  const [hide, setHide] = useState(() => { try { return localStorage.getItem('fa_hide') === '1'; } catch { return false; } });

  useEffect(() => {
    const h = () => setRoute(normalizeHash());
    const toastHandler = (e) => { setMsg(e.detail); setTimeout(() => setMsg(''), 3000); };
    window.addEventListener('hashchange', h);
    window.addEventListener('fa-toast', toastHandler);
    return () => { window.removeEventListener('hashchange', h); window.removeEventListener('fa-toast', toastHandler); };
  }, []);
  useEffect(() => {
    Promise.all([api('GET', '/accounts'), api('GET', '/categories')]).then(([accounts, categories]) => setData({ accounts, categories, loaded: true })).catch(() => setData((d) => ({ ...d, loaded: true })));
  }, [tick]);

  const ctx = useMemo(() => {
    const cat = new Map(data.categories.map((c) => [c.id, c]));
    const acc = new Map(data.accounts.map((a) => [a.id, a]));
    return {
      ...data, tick, hide,
      bump: () => setTick((x) => x + 1),
      toggleHide: () => setHide((h) => { try { localStorage.setItem('fa_hide', h ? '0' : '1'); } catch { /* ignore */ } return !h; }),
      bal: (c) => (hide ? '••••••' : money(c)),
      accName: (id) => (acc.get(id) ? accLabel(acc.get(id)) : '?'),
      accShort: (id) => acc.get(id)?.name || '?',
      accGroup: (id) => acc.get(id)?.group || '',
      catLabel: (id) => { const c = cat.get(id); return !c ? t('No category') : c.parent_id ? `${cat.get(c.parent_id)?.name} › ${c.name}` : c.name; },
      catColor: (id) => { const c = cat.get(id); const m = c?.parent_id ? cat.get(c.parent_id) : c; return m?.color || 'var(--muted)'; },
      openTx: (item, kind) => setModal({ item, kind }),
      openRec: (item) => setModal({ item, rec: true }),
      confirm: (c) => setConf(c),
      deleteTx: (tx) => (tx.recurring_id ? setDelScope(tx) : setConf({
        title: t('Delete the transaction?'), text: t('This cannot be undone.'),
        run: async () => { await api('DELETE', `/transactions/${tx.id}`); setTick((x) => x + 1); },
      })),
      deleteRec: (r) => setConf({
        title: t('Delete the recurring item?'), text: t('Transactions already created are kept.'),
        run: async () => { await api('DELETE', `/recurring/${r.id}`); setTick((x) => x + 1); },
      }),
    };
  }, [data, tick, hide]);

  if (!data.loaded) return null;
  const page = pageOf(route);
  const Page = PAGES[page] || Home;
  return html`<div class="shell">
    <nav class="tabs"><div class="brand">Finance App</div>
      ${TABS.map(([r, i, l, only]) => html`<a href=${'#/' + r} class=${(page === r || (r === 'more' && ['categories', 'recurring', 'settings'].includes(page)) ? 'on ' : '') + (only === 'd' ? 'd-only' : only === 'm' ? 'm-only' : '')}><${Icon} d=${ICONS[i]} />${t(l)}</a>`)}</nav>
    <main class=${WIDE.includes(page) ? 'wide' : ''}>${page === 'settings' ? html`<${Settings} ctx=${ctx} st=${st} reload=${reload} onLogout=${onLogout} />` : html`<${Page} ctx=${ctx} />`}</main>
    <button class="fab" aria-label=${t('New transaction')} onClick=${() => ctx.openTx(null)}>+</button>
    ${modal && html`<${FlowModal} ctx=${ctx} item=${modal.item} kind=${modal.kind} rec=${modal.rec} onClose=${() => setModal(null)} />`}
    ${conf && html`<${Confirm} title=${conf.title} text=${conf.text} onClose=${() => setConf(null)} onYes=${async () => { try { await conf.run(); } catch (e) { toast(e.message); } setConf(null); }} />`}
    ${delScope && html`<${ScopeModal} type="del" onClose=${() => setDelScope(null)}
      onPick=${async (scope) => {
        try { await api('DELETE', `/transactions/${delScope.id}?scope=${scope}`); ctx.bump(); } catch (e) { toast(e.message); }
        setDelScope(null);
      }} />`}
    ${msg && html`<div class="toast">${msg}</div>`}</div>`;
}

const LOCK_AFTER_MS = 5 * 60 * 1000;

function App() {
  const [st, setSt] = useState(null);
  const [usePassword, setUsePassword] = useState(false);
  const ref = useRef(null);
  ref.current = st;
  const lockable = (s) => !!(s && s.device && (s.pin || s.bio));

  const load = async () => {
    try {
      let s = await api('GET', '/status');
      if (s.auth && lockable(s) && !sessionStorage.getItem('fa_unlocked')) {
        // cold start with quick unlock on: ask for the PIN or biometrics
        await api('POST', '/logout');
        s = await api('GET', '/status');
      } else if (s.auth && !lockable(s)) {
        sessionStorage.setItem('fa_unlocked', '1');
      }
      configure(s.lang, s.currency);
      setUsePassword(false);
      setSt(s);
    } catch { setSt({ error: true }); }
  };

  useEffect(() => {
    load();
    const u = () => setSt((s) => (s ? { ...s, auth: false } : s));
    let hiddenAt = 0;
    const v = () => {
      if (document.hidden) { hiddenAt = Date.now(); return; }
      if (hiddenAt && Date.now() - hiddenAt > LOCK_AFTER_MS && ref.current?.auth && lockable(ref.current)) {
        sessionStorage.removeItem('fa_unlocked');
        load();
      }
      hiddenAt = 0;
    };
    window.addEventListener('fa-unauth', u);
    document.addEventListener('visibilitychange', v);
    return () => { window.removeEventListener('fa-unauth', u); document.removeEventListener('visibilitychange', v); };
  }, []);

  if (!st) return null;
  if (st.error) return html`<main><p class="err">${t('Could not reach the server.')}</p></main>`;
  if (!st.setup) return html`<${Auth} setup=${true} onDone=${load} />`;
  if (!st.auth) {
    if (lockable(st) && !usePassword) return html`<${Lock} st=${st} onDone=${load} onPassword=${() => setUsePassword(true)} />`;
    return html`<${Auth} onDone=${load} onBack=${lockable(st) ? () => setUsePassword(false) : null} />`;
  }
  // the key remounts the app when the language or currency changes, so every string and amount is redrawn
  return html`<${Shell} key=${st.lang + st.currency} st=${st} reload=${load} onLogout=${async () => { sessionStorage.removeItem('fa_unlocked'); await api('POST', '/logout'); load(); }} />`;
}

applyTheme(savedTheme());
render(html`<${App} />`, document.getElementById('app'));
if ('serviceWorker' in navigator) navigator.serviceWorker.register('/sw.js').catch(() => {});
