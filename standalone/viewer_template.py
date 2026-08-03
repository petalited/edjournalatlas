"""The embedded HTML/CSS/JS template for viewer.py's render(). Split into its own file just to
keep viewer.py's data-loading code readable -- this is a plain string constant, not logic."""

TEMPLATE = r"""<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>standalone -- local exploration viewer</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
:root {
  color-scheme: light dark;
  --space-1: 4px; --space-2: 8px; --space-3: 12px; --space-4: 16px; --space-5: 24px; --space-6: 32px;
  --radius-sm: 6px; --radius-md: 10px; --radius-lg: 16px; --radius-pill: 999px;
  --transition: 160ms ease;
  --bg: #f9f9f7; --panel: #fcfcfb; --text: #0b0b0b; --muted: #52514e; --muted-faint: #898781;
  --border: #e1e0d9; --border-strong: #c3c2b7; --accent: #2a78d6; --accent-weak: #e6f0fc;
  --good: #0ca30c; --warning: #fab219;
  --bonus-bg: #fdf1d3; --bonus-text: #7a5a00;
  --notable-bg: #efe6fb; --notable-text: #6b3fb0;
  --bad-bg: #fbe2df; --bad-text: #c22f2f;
  --row-hover: #f2f1ec;
  --shadow: 0 1px 3px rgba(11,11,11,0.08), 0 1px 2px rgba(11,11,11,0.05);
  --header-shadow: 0 2px 4px rgba(11,11,11,0.07), 0 6px 14px -8px rgba(11,11,11,0.15);
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #0d0d0d; --panel: #1a1a19; --text: #ffffff; --muted: #c3c2b7; --muted-faint: #898781;
    --border: #2c2c2a; --border-strong: #383835; --accent: #3987e5; --accent-weak: #182338;
    --good: #0ca30c; --warning: #fab219;
    --bonus-bg: #3d3110; --bonus-text: #f0c766;
    --notable-bg: #2c2140; --notable-text: #caa9f7;
    --bad-bg: #3a1f1f; --bad-text: #e66767;
    --row-hover: #211f1c;
    --shadow: 0 1px 3px rgba(0,0,0,0.5), 0 1px 2px rgba(0,0,0,0.35);
    --header-shadow: 0 2px 4px rgba(0,0,0,0.4), 0 6px 14px -8px rgba(0,0,0,0.5);
  }
}
* { box-sizing: border-box; }
body { margin: 0; font-family: -apple-system, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  background: var(--bg); color: var(--text); padding: var(--space-5); font-variant-numeric: tabular-nums; }
h1 { font-size: 1.4rem; margin: 0 0 2px; }
.subtitle { color: var(--muted); font-size: 0.85rem; margin: 0 0 var(--space-5); }
.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: var(--space-3);
  margin-bottom: var(--space-5); max-width: 900px; }
.card { background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius-md);
  padding: var(--space-3) var(--space-4); box-shadow: var(--shadow); }
.card .num { font-size: 1.3rem; font-weight: 600; }
.card .label { font-size: 0.75rem; color: var(--muted); text-transform: uppercase; letter-spacing: 0.04em; }
.controls { display: flex; flex-wrap: wrap; gap: var(--space-3); align-items: center; margin-bottom: var(--space-4);
  background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius-md);
  padding: var(--space-3) var(--space-4); box-shadow: var(--shadow); }
input[type=text], select { background: var(--bg); border: 1px solid var(--border); color: var(--text);
  border-radius: var(--radius-sm); padding: var(--space-2) var(--space-3); font-size: 0.9rem;
  transition: border-color var(--transition); }
#filterInput { width: 260px; }
button {
  background: var(--accent); color: white; border: 1px solid transparent;
  border-radius: var(--radius-sm); padding: var(--space-2) var(--space-4); font-size: 0.85rem;
  cursor: pointer; transition: background-color var(--transition), transform var(--transition), box-shadow var(--transition);
}
button:hover { background: color-mix(in srgb, var(--accent) 85%, black); }
button:active { transform: translateY(1px); }
button.secondary { background: transparent; color: var(--accent); border: 1px solid var(--accent); }
button.secondary:hover { background: var(--accent-weak); }
.checkbox-field { display: flex; align-items: center; gap: var(--space-2); font-size: 0.85rem; cursor: pointer; }
input[type=checkbox] { accent-color: var(--accent); cursor: pointer; }
.sort-arrow { font-size: 0.7rem; margin-left: 3px; opacity: 0.7; }
table { border-collapse: collapse; width: 100%; min-width: 900px; }
.panel { background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius-lg);
  overflow-x: auto; box-shadow: var(--shadow); }
th, td { padding: var(--space-2) var(--space-3); text-align: left; font-size: 0.88rem; }
thead th { position: sticky; top: 0; z-index: 1; background: var(--panel); border-bottom: 1px solid var(--border-strong);
  box-shadow: var(--header-shadow); color: var(--muted); font-weight: 600; cursor: pointer; user-select: none;
  white-space: nowrap; transition: color var(--transition); }
thead th:hover { color: var(--text); }
thead th.sorted { color: var(--accent); }
tbody tr.system-row { cursor: pointer; border-top: 1px solid var(--border); transition: background-color var(--transition); }
tbody tr.system-row:hover { background: var(--row-hover); }
tbody tr.system-row.open { background: var(--accent-weak); }
td.num, th.num { text-align: right; }
.value { font-weight: 600; }
.value.good { color: var(--good); }
.muted { color: var(--muted); }
.detail-row td { padding: 0; }
.detail-wrap { padding: var(--space-1) var(--space-3) var(--space-4) var(--space-6); background: var(--bg); }
.body-card { background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius-md);
  padding: var(--space-2) var(--space-3); width: 240px; }
.body-title { display: flex; align-items: baseline; gap: var(--space-2); }
.body-name { font-weight: 600; }
.body-type { color: var(--muted); font-weight: 400; font-size: 0.8rem; }
.body-badges { margin-top: var(--space-2); display: flex; gap: var(--space-2); flex-wrap: wrap; }
.body-grid { display: flex; flex-wrap: wrap; gap: var(--space-3); }
/* Orbit tree: left-to-right = orbital slots around the star ordered by distance, top-to-bottom
   within a column = that body's own moons ordered by distance (see moonParentKey/buildOrbitTree
   -- derived from ED's real body-naming convention: a trailing lowercase letter marks a moon of
   the preceding tokens). */
.orbit-tree { display: flex; flex-direction: row; align-items: flex-start; gap: 18px;
  overflow-x: auto; padding: var(--space-1) 2px var(--space-3); }
.tree-column { display: flex; flex-direction: column; align-items: flex-start; gap: 0; }
.tree-child { position: relative; margin-left: 20px; margin-top: var(--space-3); padding-left: var(--space-4);
  border-left: 2px solid var(--border); }
.tree-child::before { content: ''; position: absolute; left: -2px; top: 50%; width: 16px;
  border-top: 2px solid var(--border); }
.orbit-group-label { font-size: 0.78rem; color: var(--muted); margin: var(--space-3) 0 2px;
  display: flex; align-items: center; gap: 4px; }
.orbit-link { color: var(--accent); text-decoration: none; border-bottom: 1px dotted var(--accent); }
.orbit-link:hover { text-decoration: underline; }
.system-stats { display: flex; flex-wrap: wrap; gap: var(--space-3); margin: var(--space-3) 0 var(--space-4); }
.system-stats .stat { background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius-md);
  padding: var(--space-2) var(--space-3); font-size: 0.82rem; display: flex; align-items: baseline; gap: 5px;
  box-shadow: var(--shadow); }
.system-stats .stat strong { font-size: 1rem; }
.flora-list { margin: var(--space-2) 0 0; padding: 0; list-style: none; font-size: 0.85rem; }
.flora-list li { display: flex; justify-content: space-between; gap: var(--space-3); padding: 3px 0;
  border-top: 1px dashed var(--border); }
.flora-list li:first-child { border-top: none; }
.badge { display: inline-block; font-size: 0.7rem; font-weight: 600; padding: 2px var(--space-2);
  border-radius: var(--radius-pill); background: var(--bonus-bg); color: var(--bonus-text);
  border: 1px solid color-mix(in srgb, var(--bonus-text) 40%, transparent); }
.badge-sold { background: color-mix(in srgb, var(--muted) 14%, var(--panel)); color: var(--muted);
  border-color: color-mix(in srgb, var(--muted) 40%, transparent); }
.badge-lost { background: var(--bad-bg); color: var(--bad-text);
  border-color: color-mix(in srgb, var(--bad-text) 40%, transparent); }
.badge-unclaimed { background: color-mix(in srgb, var(--good) 14%, var(--panel)); color: var(--good);
  border-color: color-mix(in srgb, var(--good) 40%, transparent); }
.badge-notable { background: var(--notable-bg); color: var(--notable-text);
  border-color: color-mix(in srgb, var(--notable-text) 40%, transparent); }
.notable-chip { font-size: 0.78rem; background: var(--panel); border: 1px solid var(--border);
  border-radius: var(--radius-pill); padding: 3px var(--space-3); cursor: pointer; }
.notable-chip:hover { color: var(--accent); border-color: var(--accent); }
.pending { color: var(--muted); font-style: italic; }
.empty-note { color: var(--muted); padding: var(--space-6) var(--space-4); text-align: center; }
.empty-note-icon { font-size: 1.6rem; margin-bottom: var(--space-2); opacity: 0.7; }
.detail-header { display: flex; align-items: baseline; gap: var(--space-3); flex-wrap: wrap; margin: 0 0 var(--space-3); }
.detail-header h3 { margin: 0; font-size: 1.05rem; }
.copy-btn { background: transparent; border: 1px solid transparent; color: var(--muted);
  border-radius: var(--radius-sm); padding: 1px 5px; font-size: 0.8rem; line-height: 1.4; cursor: pointer; }
.copy-btn:hover { color: var(--accent); border-color: var(--border); }
.copy-btn.copied { color: var(--good); border-color: var(--good); }
.section-label { font-size: 0.72rem; text-transform: uppercase; letter-spacing: 0.05em; color: var(--muted);
  margin: var(--space-4) 0 var(--space-2); }
.section-label:first-child { margin-top: 0; }
.star-list { display: flex; flex-wrap: wrap; gap: var(--space-2); margin-bottom: 4px; }
.star-chip { background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius-md);
  padding: var(--space-2) var(--space-3); font-size: 0.82rem; box-shadow: var(--shadow); }
.star-chip .star-type { color: var(--muted); margin-left: 4px; }
a:focus-visible, input:focus-visible, select:focus-visible, .notable-chip:focus-visible,
button:focus-visible, thead th[data-key]:focus-visible {
  outline: 2px solid var(--accent); outline-offset: 2px; border-radius: var(--radius-sm);
}
</style>
</head>
<body>

<h1>standalone</h1>
<p class="subtitle" id="subtitle"></p>

<div class="cards" id="summaryCards"></div>
<div id="notableChips" style="margin-bottom: 16px;"></div>

<div class="controls">
  <input type="text" id="filterInput" placeholder="Search systems, body types, species...">
  <label class="checkbox-field">
    <input type="checkbox" id="notableOnlyInput">
    Only systems with a notable find
  </label>
  <label class="checkbox-field">
    <input type="checkbox" id="bioOnlyInput">
    Only systems with bio value
  </label>
  <button class="secondary" id="resetBtn">Reset filters</button>
</div>

<div class="panel">
  <table>
    <thead>
      <tr>
        <th data-key="name">System<span class="sort-arrow"></span></th>
        <th data-key="region">Region<span class="sort-arrow"></span></th>
        <th data-key="population" class="num">Population<span class="sort-arrow"></span></th>
        <th data-key="recordedBodyCount" class="num" title="Bodies with any individual scan data, out of the Discovery Scanner's true total">Bodies<span class="sort-arrow"></span></th>
        <th data-key="notableTotal" class="num">Notable<span class="sort-arrow"></span></th>
        <th data-key="firstDiscoveryCount" class="num">First discoveries<span class="sort-arrow"></span></th>
        <th data-key="bioValue" class="num">Bio value<span class="sort-arrow"></span></th>
      </tr>
    </thead>
    <tbody id="tbody"></tbody>
  </table>
  <div class="empty-note" id="emptyNote" style="display:none"><div class="empty-note-icon">🔭</div>No systems match your filters.</div>
</div>

<script id="report-data" type="application/json">__DATA_JSON__</script>
<script>
const DATA = JSON.parse(document.getElementById('report-data').textContent);

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
}
function formatCredits(n) {
  if (n === null || n === undefined) return '—';
  return n.toLocaleString('en-US') + ' cr';
}
function notableTotal(s) {
  return Object.values(s.notableCounts).reduce((a, b) => a + b, 0);
}

function edsmUrl(systemName) {
  return `https://www.edsm.net/en/system?systemName=${encodeURIComponent(systemName)}`;
}
// No per-body/per-star deep link is reliable without EDSM's own internal numeric IDs (this
// project never fetches them) -- every name links to the same system page instead, which lists
// all of a system's known bodies once EDSM has data for it. Same approach edexotracker's main
// report uses for its own Discord export.
function discordLink(label, systemName) {
  return `[${label}](${edsmUrl(systemName)})`;
}

const RARE_STAR_EMOJI = {
  'Supermassive black hole': '🕳️', 'Black hole': '🕳️', 'Neutron star': '🌠',
  'White dwarf': '⚪', 'Wolf-Rayet star': '💥', 'Herbig Ae/Be star': '🌫️',
  'Carbon star': '🟤', 'MS-type star': '🔴', 'S-type star': '🔴',
};
function starEmoji(st) {
  return RARE_STAR_EMOJI[st.notableLabel] || '⭐';
}
function bodyEmoji(b) {
  const t = b.type || '';
  if (t === 'Earthlike body') return '🌍';
  if (t === 'Water world') return '🌊';
  if (t === 'Ammonia world') return '🟢';
  if (t.includes('gas giant with')) return '🧬';
  if (t.includes('gas giant') || t === 'Water giant') return '🪐';
  if (t === 'Metal rich body') return '🔩';
  if (t === 'High metal content body' || t.includes('Rocky')) return '🪨';
  if (t === 'Icy body') return '❄️';
  return '🌑';
}

function fallbackCopy(text) {
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.style.position = 'fixed';
  ta.style.left = '-9999px';
  document.body.appendChild(ta);
  ta.select();
  try { document.execCommand('copy'); } catch (e) { /* best effort */ }
  document.body.removeChild(ta);
}
function copyWithFeedback(btn, text) {
  const showCopied = () => {
    const original = btn.textContent;
    btn.textContent = '✓';
    btn.classList.add('copied');
    setTimeout(() => { btn.textContent = original; btn.classList.remove('copied'); }, 1200);
  };
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(showCopied, () => { fallbackCopy(text); showCopied(); });
  } else {
    fallbackCopy(text);
    showCopied();
  }
}
function copyBtn(name) {
  return `<button class="copy-btn" type="button" title="Copy system name" aria-label="Copy system name" data-copy-name="${escapeHtml(name)}">📋</button>`;
}
function discordCopyBtn(name) {
  return `<button class="copy-btn discord-copy-btn" type="button" data-copy-mode="personal" title="Copy your progress for Discord (includes your bonuses)" aria-label="Copy your progress for Discord" data-copy-name="${escapeHtml(name)}">💬</button>`;
}
function discordObjectiveCopyBtn(name) {
  return `<button class="copy-btn discord-copy-btn" type="button" data-copy-mode="objective" title="Copy an objective overview for Discord (no personal bonuses -- what's actually available)" aria-label="Copy an objective overview for Discord" data-copy-name="${escapeHtml(name)}">🌐</button>`;
}

function starDiscordBits(s) {
  return s.stars.map(st => {
    const label = st.notableLabel
      ? `${st.type}${st.subclass ? st.subclass : ''}${st.luminosity ? ' ' + st.luminosity : ''} (${st.notableLabel})`
      : st.type;
    return `${starEmoji(st)} ${discordLink(label, s.name)}`;
  });
}

// A plain, non-notable body's type doesn't add much beyond what its own emoji already conveys
// -- only spell it out when it's actually notable, where the label itself is the interesting part.
function bodyNameBit(b, systemName) {
  const nameBit = `${bodyEmoji(b)} ${discordLink(`**${b.name}**`, systemName)}`;
  return b.notableLabel ? `${nameBit} — ✨ ${b.notableLabel}` : nameBit;
}

// Two Discord export variants, same reasoning as edexotracker's main report: a commander's own
// bonuses (first footfall, first-logged) are one-time opportunities -- once claimed, they're
// gone for whoever else visits, so they don't belong in a number meant to represent what's
// genuinely still "available" to share/compare.
//   - Personal ("your progress"): actual bonus-inclusive value, first-discovery bragging.
//   - Objective ("what's available"): baseValue only (pre-bonus, from the value table) --
//     what's guaranteed to be there for anyone.
function formatSystemForDiscordPersonal(s) {
  const lines = [];
  lines.push(`🌌 ${discordLink(`**${s.name}**`, s.name)}`);
  if (s.claimedByCommander) lines.push('🚩 Claimed by you (Colonisation)');
  if (s.stars.length) lines.push(`Stars: ${starDiscordBits(s).join(', ')}`);
  lines.push(`🪐 Bodies: ${s.recordedBodyCount}/${s.bodyCountTotal || s.recordedBodyCount} examined`);
  if (s.population > 0) {
    const factionBit = s.faction ? ` — controlled by ${s.faction}` : '';
    lines.push(`👥 Population: ${s.population.toLocaleString('en-US')}${factionBit}`);
  }
  if (notableTotal(s) > 0) {
    lines.push(`✨ Notable/rare: ${Object.entries(s.notableCounts).map(([label, n]) => `${n}x ${label}`).join(', ')}`);
  }
  if (s.bioValue > 0) {
    const bonus = s.bodies.reduce((sum, b) => sum + b.flora.reduce(
      (s2, f) => s2 + ((f.value && !f.lost && f.baseValue) ? (f.value - f.baseValue) : 0), 0), 0);
    const bonusBit = bonus > 0 ? ` (${formatCredits(bonus)} bonus)` : '';
    lines.push(`🧬 Exobiology: ${formatCredits(s.bioValue)}${bonusBit}`);
  }
  if (s.firstDiscoveryCount > 0) lines.push(`🌟 First discoveries: ${s.firstDiscoveryCount}`);

  const worthMentioning = s.bodies
    .filter(b => b.bioSignalCount > 0 || b.notableLabel)
    .sort((a, b) =>
      b.flora.reduce((s2, f) => s2 + (f.value && !f.lost ? f.value : 0), 0) -
      a.flora.reduce((s2, f) => s2 + (f.value && !f.lost ? f.value : 0), 0)
    );
  if (worthMentioning.length) {
    lines.push('', '**Bodies:**');
    const shown = worthMentioning.slice(0, 15);
    for (const b of shown) {
      const bits = [bodyNameBit(b, s.name)];
      if (b.bioSignalCount > 0) {
        const scanned = b.flora.filter(f => f.count === 3).length;
        bits.push(`🧬 ${scanned}/${b.bioSignalCount} scanned`);
        const unclaimed = b.flora.reduce((s2, f) => s2 + ((f.value && !f.sold && !f.lost) ? f.value : 0), 0);
        if (unclaimed > 0) bits.push(`${formatCredits(unclaimed)} unclaimed`);
      }
      if (b.discovered && b.wasDiscovered === false) bits.push('🌟 first discovery');
      lines.push('  ' + bits.join(' — '));
    }
    const remaining = worthMentioning.length - shown.length;
    if (remaining > 0) lines.push(`  _+${remaining} more_`);
  }
  return lines.join('\n');
}

function formatSystemForDiscordObjective(s) {
  const lines = [];
  lines.push(`🌌 ${discordLink(`**${s.name}**`, s.name)}`);
  if (s.stars.length) lines.push(`Stars: ${starDiscordBits(s).join(', ')}`);
  lines.push(`🪐 Bodies: ${s.recordedBodyCount}/${s.bodyCountTotal || s.recordedBodyCount} examined`);
  if (s.population > 0) {
    const factionBit = s.faction ? ` — controlled by ${s.faction}` : '';
    lines.push(`👥 Population: ${s.population.toLocaleString('en-US')}${factionBit}`);
  }
  if (notableTotal(s) > 0) {
    lines.push(`✨ Notable/rare: ${Object.entries(s.notableCounts).map(([label, n]) => `${n}x ${label}`).join(', ')}`);
  }
  const bioBase = s.bodies.reduce((sum, b) => sum + b.flora.reduce((s2, f) => s2 + (f.baseValue || 0), 0), 0);
  if (bioBase > 0) lines.push(`🧬 Exobiology available: ${formatCredits(bioBase)}`);

  const worthMentioning = s.bodies
    .filter(b => b.bioSignalCount > 0 || b.notableLabel)
    .sort((a, b) =>
      b.flora.reduce((s2, f) => s2 + (f.baseValue || 0), 0) -
      a.flora.reduce((s2, f) => s2 + (f.baseValue || 0), 0)
    );
  if (worthMentioning.length) {
    lines.push('', '**Bodies:**');
    const shown = worthMentioning.slice(0, 15);
    for (const b of shown) {
      const bits = [bodyNameBit(b, s.name)];
      if (b.bioSignalCount > 0) {
        bits.push(`🧬 ${b.bioSignalCount} signal${b.bioSignalCount === 1 ? '' : 's'}`);
        const base = b.flora.reduce((s2, f) => s2 + (f.baseValue || 0), 0);
        if (base > 0) bits.push(`${formatCredits(base)} available`);
      }
      lines.push('  ' + bits.join(' — '));
    }
    const remaining = worthMentioning.length - shown.length;
    if (remaining > 0) lines.push(`  _+${remaining} more_`);
  }
  return lines.join('\n');
}

let sortKey = 'bioValue';
let sortDir = -1;
let filterText = '';
let notableOnly = false;
let bioOnly = false;
let openSystem = null;

function systemMatchesSearch(s, needle) {
  if (!needle) return true;
  if (s.name.toLowerCase().includes(needle)) return true;
  if (s.region && s.region.toLowerCase().includes(needle)) return true;
  if (s.bodies.some(b => b.type && b.type.toLowerCase().includes(needle))) return true;
  if (s.stars.some(st => st.type && st.type.toLowerCase().includes(needle))) return true;
  if (s.bodies.some(b => b.flora.some(f =>
    (f.genus && f.genus.toLowerCase().includes(needle)) ||
    (f.species && f.species.toLowerCase().includes(needle))
  ))) return true;
  return false;
}

function sortedSystems() {
  const needle = filterText.trim().toLowerCase();
  let list = DATA.systems.filter(s => systemMatchesSearch(s, needle));
  if (notableOnly) list = list.filter(s => notableTotal(s) > 0);
  if (bioOnly) list = list.filter(s => s.bioValue > 0);
  list = list.map(s => ({...s, notableTotal: notableTotal(s)}));
  list.sort((a, b) => {
    let av = a[sortKey], bv = b[sortKey];
    if (typeof av === 'string') return sortDir * av.localeCompare(bv || '');
    return sortDir * ((av ?? 0) - (bv ?? 0));
  });
  return list;
}

function notableBadge(label) {
  return `<span class="badge badge-notable">✨ ${escapeHtml(label)}</span>`;
}

function slugify(s) {
  return s.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
}
function starAnchorId(systemName, starName) {
  return `star-${slugify(systemName)}-${slugify(starName)}`;
}

function bodyCard(b) {
  const badges = [];
  if (b.notableLabel) badges.push(notableBadge(b.notableLabel));
  if (b.discovered && b.wasDiscovered === false) badges.push('<span class="badge">🌟 First discovery</span>');
  if (b.wasFootfalled === false) badges.push('<span class="badge badge-unclaimed">First footfall available</span>');
  const floraHtml = b.flora.length ? `<ul class="flora-list">${b.flora.map(f => {
    const label = f.species || (f.genus ? f.genus + ' (undetermined)' : 'Unknown organism');
    const bits = [];
    if (f.sold) bits.push('<span class="badge badge-sold">Sold</span>');
    else if (f.lost) bits.push('<span class="badge badge-lost">Lost</span>');
    else if (f.value) bits.push('<span class="badge badge-unclaimed">Unclaimed</span>');
    if (f.footfallBonus) bits.push('<span class="badge">Footfall bonus</span>');
    if (f.firstLoggedBonus) bits.push('<span class="badge">First logged bonus</span>');
    return `<li><span>${escapeHtml(label)}</span><span>${f.value ? formatCredits(f.value) : ''} ${bits.join(' ')}</span></li>`;
  }).join('')}</ul>` : '';
  return `<div class="body-card">
    <div class="body-title"><span class="body-name">${escapeHtml(b.name)}</span><span class="body-type">${escapeHtml(b.type)}</span></div>
    ${b.bioSignalCount > 0 ? `<div class="muted" style="font-size:0.8rem;margin-top:2px">${b.bioSignalCount} bio signal${b.bioSignalCount===1?'':'s'}</div>` : ''}
    ${badges.length ? `<div class="body-badges">${badges.join('')}</div>` : ''}
    ${floraHtml}
  </div>`;
}

function starChip(sys, st) {
  const label = st.distance > 0 ? `${st.distance.toLocaleString()} ls` : 'main star';
  const notable = st.notableLabel ? ` ${notableBadge(st.notableLabel)}` : '';
  const first = st.wasDiscovered === false ? ' <span class="badge">🌟 First discovery</span>' : '';
  return `<div class="star-chip" id="${starAnchorId(sys.name, st.name)}"><strong>${escapeHtml(st.name)}</strong>
    <span class="star-type">${escapeHtml(st.type)}${st.subclass ? ' ' + st.subclass : ''}${st.luminosity ? ' ' + escapeHtml(st.luminosity) : ''} · ${label}</span>${notable}${first}
  </div>`;
}

// Groups bodies by which star they orbit (parentStars is a single nearest-star name here, or ''
// if unknown -- unlike edexotracker/report.py's list-of-circumbinary-stars, standalone only
// resolves one nearest parent per body, see docs/StandaloneJournalParser.md).
function groupBodiesByOrbit(bodies) {
  const groups = new Map();
  for (const b of bodies) {
    const key = b.parentStars || '(unknown)';
    if (!groups.has(key)) groups.set(key, { star: b.parentStars || '', bodies: [] });
    groups.get(key).bodies.push(b);
  }
  const list = [...groups.values()];
  for (const g of list) g.bodies.sort((a, b) => a.distance - b.distance);
  list.sort((a, b) => Math.min(...a.bodies.map(x => x.distance)) - Math.min(...b.bodies.map(x => x.distance)));
  return list;
}

// Real ED body-naming convention (e.g. "A 6", "A 6 a".."A 6 g"): a trailing single lowercase-
// letter token marks a moon of the body named by the tokens before it. Universal across ED, not
// tied to any particular data source.
function moonParentKey(name) {
  const tokens = name.split(' ');
  const last = tokens[tokens.length - 1];
  if (tokens.length > 1 && /^[a-z]$/.test(last)) {
    return tokens.slice(0, -1).join(' ');
  }
  return null;
}

// One column per top-level body directly orbiting the star (left-to-right = distance order),
// each with its own moons stacked underneath (top-to-bottom = distance order) -- see orbitTreeHtml.
function buildOrbitTree(bodies) {
  const byName = new Map(bodies.map(b => [b.name, b]));
  const topLevel = [];
  const moonsByParent = new Map();
  for (const b of bodies) {
    const parentKey = moonParentKey(b.name);
    if (parentKey && byName.has(parentKey)) {
      if (!moonsByParent.has(parentKey)) moonsByParent.set(parentKey, []);
      moonsByParent.get(parentKey).push(b);
    } else {
      topLevel.push(b);
    }
  }
  topLevel.sort((a, b) => a.distance - b.distance);
  return topLevel.map(b => ({
    body: b,
    moons: (moonsByParent.get(b.name) || []).sort((a, c) => a.distance - c.distance),
  }));
}

function orbitTreeHtml(bodies) {
  const columns = buildOrbitTree(bodies).map(col => {
    const moonCards = col.moons.map(m => `<div class="tree-child">${bodyCard(m)}</div>`).join('');
    return `<div class="tree-column">${bodyCard(col.body)}${moonCards}</div>`;
  }).join('');
  return `<div class="orbit-tree">${columns}</div>`;
}

function orbitGroupLabel(sys, starName) {
  if (!starName) return 'Orbiting an unknown body';
  return `Orbiting <a class="orbit-link" href="#${starAnchorId(sys.name, starName)}">${escapeHtml(starName)}</a>`;
}

function systemStatsHtml(s) {
  const stats = [
    [s.recordedBodyCount, 'bodies recorded'],
    [s.stars.length, s.stars.length === 1 ? 'star' : 'stars'],
  ];
  if (notableTotal(s) > 0) stats.push([notableTotal(s), 'notable']);
  if (s.firstDiscoveryCount > 0) stats.push([s.firstDiscoveryCount, 'first discoveries 🌟']);
  if (s.claimedByCommander) stats.push(['🚩', 'claimed (Colonisation)']);
  return '<div class="system-stats">' +
    stats.map(([num, label]) => `<div class="stat"><strong>${num}</strong><span>${escapeHtml(label)}</span></div>`).join('') +
    '</div>';
}

function render() {
  const list = sortedSystems();
  const tbody = document.getElementById('tbody');
  document.getElementById('emptyNote').style.display = list.length ? 'none' : 'block';

  tbody.innerHTML = list.map(s => {
    const isOpen = openSystem === s.name;
    const row = `<tr class="system-row ${isOpen ? 'open' : ''}" data-name="${escapeHtml(s.name)}">
      <td>${escapeHtml(s.name)}</td>
      <td class="muted">${escapeHtml(s.region || '—')}</td>
      <td class="num">${s.population.toLocaleString()}</td>
      <td class="num">${s.recordedBodyCount}</td>
      <td class="num">${s.notableTotal > 0 ? `<strong>${s.notableTotal}</strong>` : '—'}</td>
      <td class="num">${s.firstDiscoveryCount > 0 ? s.firstDiscoveryCount : '—'}</td>
      <td class="num value ${s.bioValue > 0 ? 'good' : ''}">${s.bioValue > 0 ? formatCredits(s.bioValue) : '—'}</td>
    </tr>`;
    if (!isOpen) return row;

    const starsHtml = s.stars.length
      ? `<div class="section-label">Stars</div><div class="star-list">${s.stars.map(st => starChip(s, st)).join('')}</div>` : '';
    const bodiesHtml = s.bodies.length
      ? `<div class="section-label">Bodies</div>` + groupBodiesByOrbit(s.bodies).map(g =>
          `<div class="orbit-group-label">${orbitGroupLabel(s, g.star)}</div>${orbitTreeHtml(g.bodies)}`
        ).join('')
      : '<div class="muted">No individually-scanned bodies recorded for this system.</div>';

    const detail = `<tr class="detail-row"><td colspan="7"><div class="detail-wrap">
      <div class="detail-header"><h3>${escapeHtml(s.name)}</h3>${copyBtn(s.name)}${discordCopyBtn(s.name)}${discordObjectiveCopyBtn(s.name)}${s.faction ? `<span class="muted">Controlled by ${escapeHtml(s.faction)}</span>` : ''}</div>
      ${systemStatsHtml(s)}
      ${starsHtml}
      ${bodiesHtml}
    </div></td></tr>`;
    return row + detail;
  }).join('');

  tbody.querySelectorAll('tr.system-row').forEach(tr => {
    tr.addEventListener('click', () => {
      const name = tr.getAttribute('data-name');
      openSystem = openSystem === name ? null : name;
      render();
    });
  });

  tbody.querySelectorAll('.copy-btn:not(.discord-copy-btn)').forEach(btn => {
    btn.addEventListener('click', e => {
      e.stopPropagation();
      copyWithFeedback(btn, btn.getAttribute('data-copy-name'));
    });
  });

  tbody.querySelectorAll('.discord-copy-btn').forEach(btn => {
    btn.addEventListener('click', e => {
      e.stopPropagation();
      const sys = DATA.systems.find(s => s.name === btn.getAttribute('data-copy-name'));
      if (!sys) return;
      const text = btn.getAttribute('data-copy-mode') === 'objective'
        ? formatSystemForDiscordObjective(sys)
        : formatSystemForDiscordPersonal(sys);
      copyWithFeedback(btn, text);
    });
  });

  document.querySelectorAll('thead th[data-key]').forEach(th => {
    const isSorted = th.getAttribute('data-key') === sortKey;
    th.classList.toggle('sorted', isSorted);
    const arrow = th.querySelector('.sort-arrow');
    if (arrow) arrow.textContent = isSorted ? (sortDir === 1 ? '▲' : '▼') : '';
  });
}

document.querySelectorAll('thead th[data-key]').forEach(th => {
  th.setAttribute('tabindex', '0');
  th.addEventListener('click', () => {
    const key = th.getAttribute('data-key');
    if (sortKey === key) { sortDir *= -1; } else { sortKey = key; sortDir = key === 'name' || key === 'region' ? 1 : -1; }
    render();
  });
});

document.getElementById('filterInput').addEventListener('input', e => { filterText = e.target.value; render(); });
document.getElementById('notableOnlyInput').addEventListener('change', e => { notableOnly = e.target.checked; render(); });
document.getElementById('bioOnlyInput').addEventListener('change', e => { bioOnly = e.target.checked; render(); });

document.getElementById('resetBtn').addEventListener('click', () => {
  filterText = ''; notableOnly = false; bioOnly = false; openSystem = null;
  sortKey = 'bioValue'; sortDir = -1;
  document.getElementById('filterInput').value = '';
  document.getElementById('notableOnlyInput').checked = false;
  document.getElementById('bioOnlyInput').checked = false;
  render();
});

function renderSummary() {
  const totalSystems = DATA.systems.length;
  const totalBio = DATA.systems.reduce((sum, s) => sum + s.bioValue, 0);
  const totalFirstDiscoveries = DATA.systems.reduce((sum, s) => sum + s.firstDiscoveryCount, 0);
  const notableAgg = {};
  for (const s of DATA.systems) {
    for (const [label, n] of Object.entries(s.notableCounts)) {
      notableAgg[label] = (notableAgg[label] || 0) + n;
    }
  }
  const totalNotable = Object.values(notableAgg).reduce((a, b) => a + b, 0);

  const cards = [
    [totalSystems.toLocaleString(), 'Systems visited'],
    [totalNotable.toLocaleString(), 'Notable finds'],
    [totalFirstDiscoveries.toLocaleString(), 'First discoveries'],
    [formatCredits(totalBio), 'Presumed bio value'],
  ];
  document.getElementById('summaryCards').innerHTML = cards.map(([num, label]) =>
    `<div class="card"><div class="num">${num}</div><div class="label">${escapeHtml(label)}</div></div>`
  ).join('');

  document.getElementById('notableChips').innerHTML = Object.entries(notableAgg)
    .sort((a, b) => b[1] - a[1])
    .map(([label, n]) => `<span class="notable-chip" data-label="${escapeHtml(label)}">${n}x ${escapeHtml(label)}</span> `)
    .join('');
  document.querySelectorAll('.notable-chip').forEach(chip => {
    chip.setAttribute('tabindex', '0');
    chip.addEventListener('click', () => {
      document.getElementById('filterInput').value = chip.getAttribute('data-label');
      filterText = chip.getAttribute('data-label');
      render();
    });
  });

  document.getElementById('subtitle').textContent = `Generated ${DATA.generatedAt} — ${totalSystems} systems, no ExploData/BioScan/Pioneer dependency, parsed directly from your own journal.`;
}

renderSummary();
render();
</script>
</body>
</html>
"""
