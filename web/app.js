import { html, render, useState, useEffect, useMemo } from './vendor/preact-htm.js';

/* ---------- utilità ---------- */

const api = async (method, path, body) => {
  const opt = { method, headers: {}, credentials: 'same-origin' };
  if (body !== undefined) {
    const csv = typeof body === 'string';
    opt.headers['Content-Type'] = csv ? 'text/csv' : 'application/json';
    opt.body = csv ? body : JSON.stringify(body);
  }
  const r = await fetch('/api' + path, opt);
  if (r.status === 401 && !path.startsWith('/login') && !path.startsWith('/password')) {
    window.dispatchEvent(new Event('fa-unauth'));
    throw new Error('Accesso richiesto');
  }
  if (r.status === 204) return null;
  const data = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(data.error || 'Errore');
  return data;
};

const eur = new Intl.NumberFormat('it-IT', { style: 'currency', currency: 'EUR' });
const money = (c) => eur.format((c || 0) / 100);
const sign = (kind, c) => (kind === 'income' ? '+' : kind === 'expense' ? '−' : '') + money(c);
const cents = (s) => {
  let t = String(s).replace(/\s|€/g, '');
  if (t.includes(',') && t.includes('.')) t = t.replace(/\./g, '');
  const n = parseFloat(t.replace(',', '.'));
  return Number.isFinite(n) ? Math.round(n * 100) : NaN;
};
const toInput = (c) => (c / 100).toFixed(2).replace('.', ',');
const todayStr = () => new Date().toLocaleDateString('sv');
const dayLabel = (d) => new Date(d + 'T00:00').toLocaleDateString('it-IT', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
const shortDate = (d) => new Date(d + 'T00:00').toLocaleDateString('it-IT', { day: 'numeric', month: 'short' });
const monthLabel = (y, m) => new Date(y, m - 1, 1).toLocaleDateString('it-IT', { month: 'long', year: 'numeric' });
const PALETTE = ['#0f766e', '#2563eb', '#d97706', '#9333ea', '#dc2626', '#0891b2', '#65a30d', '#db2777', '#4f46e5', '#ea580c'];
const KIND = { expense: 'Uscita', income: 'Entrata', transfer: 'Trasferimento' };
const FREQ = { weekly: 'Ogni settimana', monthly: 'Ogni mese', yearly: 'Ogni anno' };
const ACC_TYPES = ['corrente', 'contanti', 'risparmio', 'carta', 'investimenti'];
const NEEDS_MOVE = 'scegli dove spostarli';

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
};

/* ---------- componenti base ---------- */

function Modal({ title, onClose, children }) {
  useEffect(() => {
    const k = (e) => e.key === 'Escape' && onClose();
    window.addEventListener('keydown', k);
    return () => window.removeEventListener('keydown', k);
  }, []);
  return html`<div class="overlay" onMouseDown=${(e) => e.target === e.currentTarget && onClose()}>
    <div class="modal" role="dialog" aria-modal="true"><h2>${title}</h2>${children}</div></div>`;
}

function Confirm({ title, text, label = 'Elimina', onYes, onClose }) {
  const [busy, setBusy] = useState(false);
  return html`<${Modal} title=${title} onClose=${onClose}>
    <p>${text}</p>
    <div class="actions">
      <button class="btn" onClick=${onClose}>Annulla</button>
      <button class="btn primary" disabled=${busy} onClick=${async () => { setBusy(true); await onYes(); }}>${label}</button>
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
    <button class="btn small" aria-label="Precedente" onClick=${() => step(-1)}>‹</button>
    <b>${year ? ym.y : monthLabel(ym.y, ym.m)}</b>
    <button class="btn small" aria-label="Successivo" onClick=${() => step(1)}>›</button></div>`;
}

const Stats = ({ s }) => html`<div class="stats">
  <div class="stat"><small>Guadagnato</small><b class="income num">${money(s.income)}</b></div>
  <div class="stat"><small>Speso</small><b class="expense num">${money(s.expense)}</b></div>
  <div class="stat"><small>Risparmiato</small><b class=${'num ' + (s.saved < 0 ? 'expense' : '')}>${money(s.saved)}</b></div></div>`;

const Status = ({ s }) => s.error ? html`<p class="err">${s.error}</p>` : s.loading && !s.data ? html`<p class="muted">Caricamento…</p>` : null;

/* ---------- modale movimento / ricorrenza ---------- */

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
  const set = (k) => (e) => setF((x) => ({ ...x, [k]: e.target.type === 'checkbox' ? e.target.checked : e.target.value }));
  const opts = useMemo(() => catOptions(ctx.categories, f.kind), [ctx.categories, f.kind]);
  const valid = opts.some((o) => String(o.id) === String(f.category_id));

  if (!accounts.length) {
    return html`<${Modal} title="Nessun conto" onClose=${onClose}><p>Crea prima un conto dalla sezione Conti.</p>
      <div class="actions"><button class="btn primary" onClick=${() => { onClose(); location.hash = '#/conti'; }}>Vai ai conti</button></div></${Modal}>`;
  }

  const save = async () => {
    const amount = cents(f.amount);
    if (!(amount > 0)) return setErr('Inserisci un importo maggiore di zero');
    const body = {
      kind: f.kind, amount_cents: amount, account_id: Number(f.account_id), note: f.note,
      to_account_id: f.kind === 'transfer' ? Number(f.to_account_id) || null : null,
      category_id: f.kind !== 'transfer' && valid ? Number(f.category_id) : null,
    };
    if (rec) Object.assign(body, { freq: f.freq, start_date: f.date, end_date: f.end_date, active: f.active });
    else body.date = f.date;
    setBusy(true);
    try {
      const path = rec ? '/recurring' : '/transactions';
      await (item ? api('PUT', `${path}/${item.id}`, body) : api('POST', path, body));
      localStorage.setItem('fa_acc', f.account_id);
      ctx.bump();
      onClose();
    } catch (e) { setErr(e.message); setBusy(false); }
  };
  const del = () => ctx.confirm({
    title: rec ? 'Eliminare la ricorrenza?' : 'Eliminare il movimento?',
    text: rec ? 'I movimenti già generati restano.' : 'L\'operazione non si può annullare.',
    run: async () => { await api('DELETE', `${rec ? '/recurring' : '/transactions'}/${item.id}`); ctx.bump(); onClose(); },
  });

  const title = (item ? 'Modifica ' : 'Nuov' + (rec ? 'a ' : 'o ')) + (rec ? 'ricorrenza' : 'movimento');
  return html`<${Modal} title=${title} onClose=${onClose}>
    <div class="seg" style="margin-bottom:14px">
      ${Object.entries(KIND).map(([k, v]) => html`<button type="button" class=${f.kind === k ? 'on' : ''} onClick=${() => setF({ ...f, kind: k, category_id: '' })}>${v}</button>`)}
    </div>
    <div class="two">
      <label class="f">Importo (€)<input inputmode="decimal" placeholder="0,00" value=${f.amount} onInput=${set('amount')} autofocus /></label>
      <label class="f">${rec ? 'Prima data' : 'Data'}<input type="date" value=${f.date} onInput=${set('date')} /></label>
    </div>
    <div class=${f.kind === 'transfer' ? 'two' : ''}>
      <label class="f">${f.kind === 'transfer' ? 'Da conto' : 'Conto'}
        <select value=${f.account_id} onChange=${set('account_id')}>${accounts.map((a) => html`<option value=${a.id}>${a.name}</option>`)}</select></label>
      ${f.kind === 'transfer' && html`<label class="f">A conto
        <select value=${f.to_account_id} onChange=${set('to_account_id')}><option value="">—</option>
          ${accounts.filter((a) => String(a.id) !== String(f.account_id)).map((a) => html`<option value=${a.id}>${a.name}</option>`)}</select></label>`}
    </div>
    ${f.kind !== 'transfer' && html`<label class="f">Categoria
      <select value=${valid ? f.category_id : ''} onChange=${set('category_id')}><option value="">Senza categoria</option>
        ${opts.map((o) => html`<option value=${o.id}>${o.label}</option>`)}</select></label>`}
    <label class="f">Nota<input value=${f.note} onInput=${set('note')} maxlength="200" /></label>
    ${rec && html`<div class="two">
      <label class="f">Frequenza<select value=${f.freq} onChange=${set('freq')}>${Object.entries(FREQ).map(([k, v]) => html`<option value=${k}>${v}</option>`)}</select></label>
      <label class="f">Fine (opzionale)<input type="date" value=${f.end_date} onInput=${set('end_date')} /></label></div>
      <label class="check"><input type="checkbox" checked=${f.active} onChange=${set('active')} /> Attiva</label>`}
    ${err && html`<p class="err">${err}</p>`}
    <div class="actions">
      ${item && html`<button class="btn danger left" onClick=${del}>Elimina</button>`}
      <button class="btn" onClick=${onClose}>Annulla</button>
      <button class="btn primary" disabled=${busy} onClick=${save}>Salva</button>
    </div></${Modal}>`;
}

/* ---------- righe movimento ---------- */

function TxRow({ ctx, t }) {
  const transfer = t.kind === 'transfer';
  const title = transfer ? `${ctx.accName(t.account_id)} → ${ctx.accName(t.to_account_id)}` : ctx.catLabel(t.category_id);
  return html`<div class="row click" onClick=${() => ctx.openTx(t)}>
    <span class="dot" style=${'background:' + (transfer ? 'var(--transfer)' : ctx.catColor(t.category_id))}></span>
    <div class="grow"><div>${title}${t.recurring_id && html`<span class="pill">ricorrente</span>`}</div>
      <div class="muted" style="font-size:.85rem">${[!transfer && ctx.accName(t.account_id), t.note].filter(Boolean).join(' · ') || ' '}</div></div>
    <b class=${'num ' + t.kind}>${sign(t.kind, t.amount_cents)}</b></div>`;
}

function TxList({ ctx, list, dates = true }) {
  if (!list.length) return html`<div class="empty">Nessun movimento</div>`;
  let last = '';
  return list.map((t) => {
    const head = dates && t.date !== last ? html`<div class="day">${dayLabel(t.date)}</div>` : null;
    last = t.date;
    return html`<${Fragment}>${head}<${TxRow} ctx=${ctx} t=${t} /></${Fragment}>`;
  });
}
const Fragment = ({ children }) => children;

/* ---------- pagine ---------- */

function Home({ ctx }) {
  const now = new Date();
  const [ym, setYm] = useState({ y: now.getFullYear(), m: now.getMonth() + 1 });
  const sum = useLoad(() => api('GET', `/summary?year=${ym.y}&month=${ym.m}`), [ym, ctx.tick]);
  const recent = useLoad(() => api('GET', '/transactions?limit=8'), [ctx.tick]);
  const live = ctx.accounts.filter((a) => !a.archived);
  const total = live.reduce((s, a) => s + a.balance_cents, 0);
  return html`<div>
    <div class="head"><h1>Home</h1><${MonthNav} ym=${ym} setYm=${setYm} /></div>
    <${Status} s=${sum} />
    ${sum.data && html`<${Stats} s=${sum.data} />`}
    <div class="card"><h2>Conti <span class="muted num" style="float:right;font-weight:400">${money(total)}</span></h2>
      ${live.length ? live.map((a) => html`<div class="row"><div class="grow"><div>${a.name}</div><div class="muted" style="font-size:.85rem">${a.type}</div></div>
        <b class=${'num ' + (a.balance_cents < 0 ? 'expense' : '')}>${money(a.balance_cents)}</b></div>`)
        : html`<div class="empty">Nessun conto. <a href="#/conti">Creane uno</a></div>`}</div>
    <div class="card"><h2>Ultimi movimenti</h2>${recent.data ? html`<${TxList} ctx=${ctx} list=${recent.data} dates=${false} />` : html`<${Status} s=${recent} />`}</div></div>`;
}

function Movimenti({ ctx }) {
  const [f, setF] = useState({ month: '', account: '', category: '', kind: '' });
  const [limit, setLimit] = useState(100);
  const set = (k) => (e) => { setF({ ...f, [k]: e.target.value }); setLimit(100); };
  const q = new URLSearchParams({ limit });
  if (f.month) { q.set('from', f.month + '-01'); q.set('to', f.month + '-31'); }
  for (const k of ['account', 'category', 'kind']) if (f[k]) q.set(k, f[k]);
  const res = useLoad(() => api('GET', '/transactions?' + q), [q.toString(), ctx.tick]);
  const macros = ctx.categories.filter((c) => !c.parent_id);
  return html`<div>
    <div class="head"><h1>Movimenti</h1></div>
    <div class="filters">
      <input type="month" value=${f.month} onInput=${set('month')} aria-label="Mese" />
      <select value=${f.account} onChange=${set('account')}><option value="">Tutti i conti</option>${ctx.accounts.map((a) => html`<option value=${a.id}>${a.name}</option>`)}</select>
      <select value=${f.category} onChange=${set('category')}><option value="">Tutte le categorie</option>${macros.map((c) => html`<option value=${c.id}>${c.name}</option>`)}</select>
      <select value=${f.kind} onChange=${set('kind')}><option value="">Tutti i tipi</option>${Object.entries(KIND).map(([k, v]) => html`<option value=${k}>${v}</option>`)}</select></div>
    <${Status} s=${res} />
    ${res.data && html`<div class="card"><${TxList} ctx=${ctx} list=${res.data} />
      ${res.data.length >= limit && html`<div style="text-align:center;padding-top:10px"><button class="btn" onClick=${() => setLimit(limit + 100)}>Mostra altri</button></div>`}</div>`}</div>`;
}

function Donut({ items, total }) {
  let acc = 0;
  return html`<svg viewBox="0 0 42 42" role="img" aria-label="Spese per categoria">
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
  const items = list.map((c, i) => ({ ...c, color: c.color || PALETTE[i % PALETTE.length] }));
  if (!items.length) return html`<div class="empty">Nessun dato</div>`;
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
  const L = 'GFMAMGLASOND';
  return html`<svg class="bars" viewBox=${`0 0 ${W} ${H}`} role="img" aria-label="Entrate e spese per mese">
    <line x1="0" x2=${W} y1=${base} y2=${base} stroke="var(--line)" />
    ${months.map((m, i) => html`<g>
      <rect x=${i * gw + 5} y=${base - h(m.income)} width="9" height=${h(m.income)} rx="2" fill="var(--income)" />
      <rect x=${i * gw + 15} y=${base - h(m.expense)} width="9" height=${h(m.expense)} rx="2" fill="var(--expense)" />
      <text x=${i * gw + gw / 2} y=${base + 14} text-anchor="middle">${L[i]}</text></g>`)}</svg>`;
}

function Riepilogo({ ctx }) {
  const now = new Date();
  const [year, setYear] = useState(false);
  const [ym, setYm] = useState({ y: now.getFullYear(), m: now.getMonth() + 1 });
  const cur = year ? { y: ym.y, m: 0 } : ym;
  const res = useLoad(() => api('GET', `/summary?year=${cur.y}&month=${cur.m}`), [cur.y, cur.m, ctx.tick]);
  const toggle = (y) => { setYear(y); if (!y && !ym.m) setYm({ y: ym.y, m: now.getMonth() + 1 }); };
  const s = res.data;
  return html`<div>
    <div class="head"><h1>Riepilogo</h1>
      <div class="seg"><button class=${!year ? 'on' : ''} onClick=${() => toggle(false)}>Mese</button><button class=${year ? 'on' : ''} onClick=${() => toggle(true)}>Anno</button></div></div>
    <div class="head"><${MonthNav} ym=${cur} setYm=${setYm} year=${year} /></div>
    <${Status} s=${res} />
    ${s && html`<${Stats} s=${s} />
      <div class="card"><h2>Spese per categoria</h2><${CatBreakdown} list=${s.expenses} total=${s.expense} kind="expense" /></div>
      <div class="card"><h2>Entrate per categoria</h2><${CatBreakdown} list=${s.incomes} total=${s.income} kind="income" /></div>
      <div class="card"><h2>Andamento ${cur.y}</h2><${Bars} months=${s.months} />
        <div class="muted" style="font-size:.8rem;text-align:center"><span class="income">■</span> entrate &nbsp; <span class="expense">■</span> spese</div></div>`}</div>`;
}

function AccountModal({ ctx, item, onClose }) {
  const [f, setF] = useState(item ? { name: item.name, type: item.type, initial: toInput(item.initial_cents), archived: item.archived } : { name: '', type: 'corrente', initial: '', archived: false });
  const [err, setErr] = useState('');
  const set = (k) => (e) => setF({ ...f, [k]: e.target.type === 'checkbox' ? e.target.checked : e.target.value });
  const save = async () => {
    const init = f.initial.trim() === '' ? 0 : cents(f.initial);
    if (Number.isNaN(init)) return setErr('Saldo iniziale non valido');
    const body = { name: f.name, type: f.type, initial_cents: init, archived: f.archived };
    try { await (item ? api('PUT', `/accounts/${item.id}`, body) : api('POST', '/accounts', body)); ctx.bump(); onClose(); } catch (e) { setErr(e.message); }
  };
  const del = () => ctx.confirm({
    title: `Eliminare «${item.name}»?`, text: 'Si può eliminare solo un conto senza movimenti; altrimenti archivialo.',
    run: async () => { await api('DELETE', `/accounts/${item.id}`); ctx.bump(); onClose(); },
  });
  return html`<${Modal} title=${item ? 'Modifica conto' : 'Nuovo conto'} onClose=${onClose}>
    <label class="f">Nome<input value=${f.name} onInput=${set('name')} autofocus /></label>
    <div class="two">
      <label class="f">Tipo<select value=${f.type} onChange=${set('type')}>${ACC_TYPES.map((t) => html`<option>${t}</option>`)}</select></label>
      <label class="f">Saldo iniziale (€)<input inputmode="decimal" placeholder="0,00" value=${f.initial} onInput=${set('initial')} /></label></div>
    ${item && html`<label class="check"><input type="checkbox" checked=${f.archived} onChange=${set('archived')} /> Archiviato (nascosto dai nuovi movimenti)</label>`}
    ${err && html`<p class="err">${err}</p>`}
    <div class="actions">${item && html`<button class="btn danger left" onClick=${del}>Elimina</button>`}
      <button class="btn" onClick=${onClose}>Annulla</button><button class="btn primary" onClick=${save}>Salva</button></div></${Modal}>`;
}

function Conti({ ctx }) {
  const [edit, setEdit] = useState(null);
  const live = ctx.accounts.filter((a) => !a.archived), old = ctx.accounts.filter((a) => a.archived);
  const total = live.reduce((s, a) => s + a.balance_cents, 0);
  const rows = (l) => l.map((a) => html`<div class="row click" onClick=${() => setEdit(a)}>
    <div class="grow"><div>${a.name}</div><div class="muted" style="font-size:.85rem">${a.type}</div></div>
    <b class=${'num ' + (a.balance_cents < 0 ? 'expense' : '')}>${money(a.balance_cents)}</b></div>`);
  return html`<div>
    <div class="head"><h1>Conti</h1><div style="display:flex;gap:8px">
      <button class="btn" onClick=${() => ctx.openTx(null, 'transfer')} disabled=${live.length < 2}>Trasferisci</button>
      <button class="btn primary" onClick=${() => setEdit({})}>Nuovo conto</button></div></div>
    <div class="card"><h2>Totale <span class="num" style="float:right">${money(total)}</span></h2>
      ${live.length ? rows(live) : html`<div class="empty">Crea il tuo primo conto</div>`}</div>
    ${old.length > 0 && html`<div class="card"><h2 class="muted">Archiviati</h2>${rows(old)}</div>`}
    ${edit && html`<${AccountModal} ctx=${ctx} item=${edit.id ? edit : null} onClose=${() => setEdit(null)} />`}</div>`;
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
  const title = item ? 'Rinomina' : parent ? `Nuova sottocategoria di ${parent.name}` : kind === 'expense' ? 'Nuova categoria di spesa' : 'Nuova categoria di entrata';
  return html`<${Modal} title=${title} onClose=${onClose}>
    <label class="f">Nome<input value=${name} onInput=${(e) => setName(e.target.value)} onKeyDown=${(e) => e.key === 'Enter' && save()} autofocus /></label>
    ${err && html`<p class="err">${err}</p>`}
    <div class="actions"><button class="btn" onClick=${onClose}>Annulla</button><button class="btn primary" onClick=${save}>Salva</button></div></${Modal}>`;
}

function MoveModal({ ctx, cat, onClose }) {
  const tree = new Set([cat.id, ...ctx.categories.filter((c) => c.parent_id === cat.id).map((c) => c.id)]);
  const opts = catOptions(ctx.categories, cat.kind).filter((o) => !tree.has(o.id));
  const [to, setTo] = useState(opts[0]?.id || '');
  const [err, setErr] = useState('');
  const go = async () => {
    try { await api('DELETE', `/categories/${cat.id}?move_to=${to}`); ctx.bump(); onClose(); } catch (e) { setErr(e.message); }
  };
  return html`<${Modal} title=${`Eliminare «${cat.name}»`} onClose=${onClose}>
    <p>Ci sono movimenti o ricorrenze in questa categoria. Scegli dove spostarli.</p>
    ${opts.length ? html`<label class="f">Sposta in<select value=${to} onChange=${(e) => setTo(e.target.value)}>${opts.map((o) => html`<option value=${o.id}>${o.label}</option>`)}</select></label>`
      : html`<p class="err">Crea prima un'altra categoria dello stesso tipo.</p>`}
    ${err && html`<p class="err">${err}</p>`}
    <div class="actions"><button class="btn" onClick=${onClose}>Annulla</button><button class="btn primary" disabled=${!opts.length} onClick=${go}>Sposta ed elimina</button></div></${Modal}>`;
}

function Categorie({ ctx }) {
  const [kind, setKind] = useState('expense');
  const [modal, setModal] = useState(null);
  const macros = ctx.categories.filter((c) => !c.parent_id && c.kind === kind);
  const remove = (c) => ctx.confirm({
    title: `Eliminare «${c.name}»?`, text: c.parent_id ? '' : 'Verranno eliminate anche le sue sottocategorie.',
    run: async () => {
      try { await api('DELETE', `/categories/${c.id}`); ctx.bump(); }
      catch (e) { if (e.message.includes(NEEDS_MOVE)) setModal({ move: c }); else toast(e.message); }
    },
  });
  const btns = (c, withSub) => html`<span style="display:flex;gap:4px">
    ${withSub && html`<button class="btn small" onClick=${() => setModal({ parent: c, kind })}>+ Sotto</button>`}
    <button class="btn small" onClick=${() => setModal({ item: c, kind })}>Modifica</button>
    <button class="btn small danger" aria-label="Elimina" onClick=${() => remove(c)}>✕</button></span>`;
  return html`<div>
    <div class="head"><h1>Categorie</h1>
      <div class="seg"><button class=${kind === 'expense' ? 'on' : ''} onClick=${() => setKind('expense')}>Spese</button><button class=${kind === 'income' ? 'on' : ''} onClick=${() => setKind('income')}>Entrate</button></div></div>
    <div class="card">
      ${macros.length ? macros.map((m, i) => html`<div>
        <div class="row"><span class="dot" style=${'background:' + (m.color || PALETTE[i % PALETTE.length])}></span><div class="grow"><div><b>${m.name}</b></div></div>${btns(m, true)}</div>
        ${ctx.categories.filter((c) => c.parent_id === m.id).map((s) => html`<div class="row subrow"><div class="grow"><div>${s.name}</div></div>${btns(s, false)}</div>`)}</div>`)
        : html`<div class="empty">Nessuna categoria</div>`}
      <div style="padding-top:10px"><button class="btn primary" onClick=${() => setModal({ kind })}>Nuova categoria</button></div></div>
    ${modal?.move ? html`<${MoveModal} ctx=${ctx} cat=${modal.move} onClose=${() => setModal(null)} />`
      : modal && html`<${CatModal} ctx=${ctx} state=${modal} onClose=${() => setModal(null)} />`}</div>`;
}

function Ricorrenti({ ctx }) {
  const res = useLoad(() => api('GET', '/recurring'), [ctx.tick]);
  const list = res.data || [];
  const toggle = async (r) => { try { await api('PUT', `/recurring/${r.id}`, { ...r, active: !r.active }); ctx.bump(); } catch (e) { toast(e.message); } };
  return html`<div>
    <div class="head"><h1>Ricorrenti</h1><button class="btn primary" onClick=${() => ctx.openRec(null)}>Nuova</button></div>
    <${Status} s=${res} />
    <div class="card">${list.length ? list.map((r) => html`<div class="row">
      <div class="grow click" style="cursor:pointer" onClick=${() => ctx.openRec(r)}>
        <div>${r.kind === 'transfer' ? `${ctx.accName(r.account_id)} → ${ctx.accName(r.to_account_id)}` : ctx.catLabel(r.category_id)}${!r.active && html`<span class="pill">in pausa</span>`}</div>
        <div class="muted" style="font-size:.85rem">${FREQ[r.freq]} · ${r.active ? 'prossima ' + shortDate(r.next_date) : 'ferma'}${r.note ? ' · ' + r.note : ''}</div></div>
      <b class=${'num ' + r.kind}>${sign(r.kind, r.amount_cents)}</b>
      <button class="btn small" onClick=${() => toggle(r)}>${r.active ? 'Pausa' : 'Riprendi'}</button></div>`)
      : !res.loading && html`<div class="empty">Nessuna ricorrenza. Aggiungi affitto, stipendio, abbonamenti…</div>`}</div></div>`;
}

function Impostazioni({ ctx, version, onLogout }) {
  const [pw, setPw] = useState({ current: '', next: '' });
  const [msg, setMsg] = useState('');
  const change = async () => {
    try { await api('POST', '/password', { current: pw.current, new: pw.next }); setPw({ current: '', next: '' }); setMsg('Password aggiornata'); }
    catch (e) { setMsg(e.message); }
  };
  const importCsv = async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    try { const r = await api('POST', '/import', await file.text()); toast(`Importati ${r.imported} movimenti`); ctx.bump(); }
    catch (er) { toast(er.message); }
    e.target.value = '';
  };
  return html`<div>
    <div class="head"><h1>Impostazioni</h1></div>
    <div class="card"><h2>Dati</h2>
      <p class="muted">Il CSV ha le colonne: data, tipo, importo, conto, conto_destinazione, categoria, sottocategoria, nota. Conti e categorie mancanti vengono creati.</p>
      <div style="display:flex;gap:8px;flex-wrap:wrap">
        <a class="btn" href="/api/export.csv" download>Esporta CSV</a>
        <label class="btn" style="cursor:pointer">Importa CSV<input type="file" accept=".csv,text/csv" hidden onChange=${importCsv} /></label>
        <a class="btn" href="/api/backup" download>Scarica backup</a></div></div>
    <div class="card"><h2>Cambia password</h2>
      <label class="f">Password attuale<input type="password" autocomplete="current-password" value=${pw.current} onInput=${(e) => setPw({ ...pw, current: e.target.value })} /></label>
      <label class="f">Nuova password (min. 6 caratteri)<input type="password" autocomplete="new-password" value=${pw.next} onInput=${(e) => setPw({ ...pw, next: e.target.value })} /></label>
      ${msg && html`<p class="muted">${msg}</p>`}
      <button class="btn primary" onClick=${change}>Aggiorna</button></div>
    <div class="card"><button class="btn" onClick=${onLogout}>Esci</button> <span class="muted" style="margin-left:8px">Versione ${version}</span></div></div>`;
}

function Altro() {
  const items = [['categorie', 'tag', 'Categorie'], ['ricorrenti', 'repeat', 'Ricorrenti'], ['impostazioni', 'gear', 'Impostazioni']];
  return html`<div><div class="head"><h1>Altro</h1></div><div class="card">
    ${items.map(([r, i, l]) => html`<a class="row" href=${'#/' + r} style="text-decoration:none;color:inherit"><${Icon} d=${ICONS[i]} /><div class="grow">${l}</div><span class="muted">›</span></a>`)}</div></div>`;
}

/* ---------- accesso ---------- */

function Auth({ setup, onDone }) {
  const [pw, setPw] = useState('');
  const [pw2, setPw2] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);
  const go = async (e) => {
    e.preventDefault();
    if (setup && pw !== pw2) return setErr('Le password non coincidono');
    setBusy(true);
    try { await api('POST', setup ? '/setup' : '/login', { password: pw }); onDone(); } catch (er) { setErr(er.message); setBusy(false); }
  };
  return html`<form class="auth" onSubmit=${go}>
    <h1>Finance App</h1>
    <p class="muted">${setup ? 'Scegli una password per proteggere i tuoi dati.' : 'Inserisci la password.'}</p>
    <label class="f">Password<input type="password" autocomplete=${setup ? 'new-password' : 'current-password'} value=${pw} onInput=${(e) => setPw(e.target.value)} autofocus /></label>
    ${setup && html`<label class="f">Ripeti la password<input type="password" autocomplete="new-password" value=${pw2} onInput=${(e) => setPw2(e.target.value)} /></label>`}
    ${err && html`<p class="err">${err}</p>`}
    <button class="btn primary" style="width:100%" disabled=${busy}>${setup ? 'Inizia' : 'Entra'}</button></form>`;
}

/* ---------- app ---------- */

const TABS = [
  ['home', 'home', 'Home'], ['movimenti', 'list', 'Movimenti'], ['riepilogo', 'chart', 'Riepilogo'], ['conti', 'wallet', 'Conti'],
  ['categorie', 'tag', 'Categorie', 'd'], ['ricorrenti', 'repeat', 'Ricorrenti', 'd'], ['impostazioni', 'gear', 'Impostazioni', 'd'], ['altro', 'more', 'Altro', 'm'],
];

function Shell({ version, onLogout }) {
  const [route, setRoute] = useState(() => location.hash.slice(2) || 'home');
  const [data, setData] = useState({ accounts: [], categories: [], loaded: false });
  const [tick, setTick] = useState(0);
  const [modal, setModal] = useState(null);
  const [conf, setConf] = useState(null);
  const [msg, setMsg] = useState('');

  useEffect(() => {
    const h = () => setRoute(location.hash.slice(2) || 'home');
    const t = (e) => { setMsg(e.detail); setTimeout(() => setMsg(''), 3000); };
    window.addEventListener('hashchange', h);
    window.addEventListener('fa-toast', t);
    return () => { window.removeEventListener('hashchange', h); window.removeEventListener('fa-toast', t); };
  }, []);
  useEffect(() => {
    Promise.all([api('GET', '/accounts'), api('GET', '/categories')]).then(([accounts, categories]) => setData({ accounts, categories, loaded: true })).catch(() => setData((d) => ({ ...d, loaded: true })));
  }, [tick]);

  const ctx = useMemo(() => {
    const cat = new Map(data.categories.map((c) => [c.id, c]));
    const acc = new Map(data.accounts.map((a) => [a.id, a]));
    return {
      ...data, tick,
      bump: () => setTick((x) => x + 1),
      accName: (id) => acc.get(id)?.name || '?',
      catLabel: (id) => { const c = cat.get(id); return !c ? 'Senza categoria' : c.parent_id ? `${cat.get(c.parent_id)?.name} › ${c.name}` : c.name; },
      catColor: (id) => { const c = cat.get(id); const m = c?.parent_id ? cat.get(c.parent_id) : c; return m?.color || 'var(--muted)'; },
      openTx: (item, kind) => setModal({ item, kind }),
      openRec: (item) => setModal({ item, rec: true }),
      confirm: (c) => setConf(c),
    };
  }, [data, tick]);

  if (!data.loaded) return null;
  const page = route.split('/')[0];
  const Page = { home: Home, movimenti: Movimenti, riepilogo: Riepilogo, conti: Conti, categorie: Categorie, ricorrenti: Ricorrenti, altro: Altro }[page];
  return html`<div class="shell">
    <nav class="tabs"><div class="brand">Finance App</div>
      ${TABS.map(([r, i, l, only]) => html`<a href=${'#/' + r} class=${(page === r || (r === 'altro' && ['categorie', 'ricorrenti', 'impostazioni'].includes(page)) ? 'on ' : '') + (only === 'd' ? 'd-only' : only === 'm' ? 'm-only' : '')}><${Icon} d=${ICONS[i]} />${l}</a>`)}</nav>
    <main>${page === 'impostazioni' ? html`<${Impostazioni} ctx=${ctx} version=${version} onLogout=${onLogout} />` : html`<${Page || Home} ctx=${ctx} />`}</main>
    <button class="fab" aria-label="Nuovo movimento" onClick=${() => ctx.openTx(null)}>+</button>
    ${modal && html`<${FlowModal} ctx=${ctx} item=${modal.item} kind=${modal.kind} rec=${modal.rec} onClose=${() => setModal(null)} />`}
    ${conf && html`<${Confirm} title=${conf.title} text=${conf.text} onClose=${() => setConf(null)} onYes=${async () => { try { await conf.run(); } catch (e) { toast(e.message); } setConf(null); }} />`}
    ${msg && html`<div class="toast">${msg}</div>`}</div>`;
}

function App() {
  const [st, setSt] = useState(null);
  const load = () => api('GET', '/status').then(setSt).catch(() => setSt({ error: true }));
  useEffect(() => {
    load();
    const u = () => setSt((s) => (s ? { ...s, auth: false } : s));
    window.addEventListener('fa-unauth', u);
    return () => window.removeEventListener('fa-unauth', u);
  }, []);
  if (!st) return null;
  if (st.error) return html`<main><p class="err">Impossibile contattare il server.</p></main>`;
  if (!st.setup) return html`<${Auth} setup=${true} onDone=${load} />`;
  if (!st.auth) return html`<${Auth} onDone=${load} />`;
  return html`<${Shell} version=${st.version} onLogout=${async () => { await api('POST', '/logout'); load(); }} />`;
}

render(html`<${App} />`, document.getElementById('app'));
if ('serviceWorker' in navigator) navigator.serviceWorker.register('/sw.js').catch(() => {});
