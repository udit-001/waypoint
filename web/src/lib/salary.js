// parseSalary turns a free-text salary string into a monthly figure
// (in thousands) plus a currency symbol, ready for the dashboard chart.
// Single source of truth for: number extraction, "OR"-alternative
// resolution, lakh-magnitude handling, k-unit conversion, and
// annual→monthly normalisation. Callers never touch the arithmetic.

const LAKH = 100000;

// detectLakh returns true when the string carries a lakh magnitude hint
// (LPA or lakh). Bare "PA" (per annum) is NOT lakh — it signals the
// period only, not the multiplier.
function detectLakh(str) {
  return /LPA|lakh/i.test(str);
}

// detectCurrency returns the symbol for the salary's currency, or null
// when no currency signal is present.
function detectCurrency(str) {
  if (/Rs\.?/i.test(str) || /₹/.test(str) || /\+\s*HRA/i.test(str) || /\d{1,2},\d{3}/.test(str) || /LPA|lakh/i.test(str)) return '₹';
  if (/\$/.test(str)) return '$';
  return null;
}

// detectUnit classifies a salary string as monthly or annual.
// Explicit annual qualifiers (LPA, lakh, /yr, /year, PA) are checked
// first — they override monthly defaults like Rs. or ₹.
function detectUnit(str) {
  if (/LPA|lakh|\/yr|\/year|\bPA\b/i.test(str)) return 'annual';
  if (/Rs\./i.test(str) || /\+\s*HRA/i.test(str)) return 'monthly';
  if (/\d{1,2},\d{3}/.test(str)) return 'monthly';
  if (/₹/.test(str)) return 'monthly';
  if (/\$/.test(str)) return 'annual';
  if (/\d+\s*k/i.test(str)) return 'annual';
  return 'annual';
}

// parseRaw extracts {low, high, mid} in k-units from a single (non-OR)
// salary string. Lakh inputs are multiplied by 100,000 before the
// k-conversion (÷1000). Returns null when nothing parses.
function parseRaw(str) {
  // Clean: strip currency symbols, thousand-separator commas, suffixes like "+ HRA"
  let clean = str
    .replace(/Rs\.?\s*/gi, '')
    .replace(/\$\s*/g, '')
    .replace(/,\s*/g, '')
    .replace(/\s*\+.*/g, '')
    .trim();

  const lakh = detectLakh(str);

  // k-format first (e.g. "70k-100k", "$100k") — already in k-units
  const kMatch = clean.match(/(\d+)\s*k/gi);
  if (kMatch) {
    const nums = kMatch.map(s => parseInt(s.match(/(\d+)/)[1]));
    if (nums.length >= 2) {
      return { low: nums[0], high: nums[1], mid: Math.round((nums[0] + nums[1]) / 2) };
    }
    return { low: nums[0], high: nums[0], mid: nums[0] };
  }

  const allNums = clean.match(/\d+/g);
  if (!allNums) return null;

  // Range like "25000-35000" or "12-15 LPA"
  const rangeMatch = clean.match(/(\d+)\s*[-–]\s*(\d+)/);
  if (rangeMatch) {
    let low = parseInt(rangeMatch[1], 10);
    let high = parseInt(rangeMatch[2], 10);
    if (lakh) { low *= LAKH; high *= LAKH; }
    if (low > 1000) low = Math.round(low / 1000);
    if (high > 1000) high = Math.round(high / 1000);
    if (low > high) [low, high] = [high, low];
    return { low, high, mid: Math.round((low + high) / 2) };
  }

  // Single number: pick the most salary-like candidate (largest value)
  const vals = allNums.map(s => parseInt(s, 10));
  let val = Math.max(...vals);
  if (lakh) val *= LAKH;
  if (val > 1000) val = Math.round(val / 1000);
  return { low: val, high: val, mid: val };
}

// parseSalary returns the monthly-normalised salary {low, high, mid, currency}
// in k-units, or null for empty/unparseable input. Annual figures are
// divided by 12; lakh magnitudes (LPA/lakh) are multiplied by 100,000 first.
export function parseSalary(str) {
  if (!str) return null;

  // Handle "OR" patterns (e.g. "Rs. 37,000 + HRA OR Rs. 31,000 + HRA")
  if (/\s+OR\s+/i.test(str)) {
    const parts = str.split(/\s+OR\s+/i);
    const parsedOptions = parts.map(p => parseSalary(p)).filter(p => p && p.mid > 0);
    if (parsedOptions.length === 0) return null;
    const lows = parsedOptions.map(p => p.low);
    const highs = parsedOptions.map(p => p.high);
    const best = parsedOptions.reduce((a, b) => a.mid > b.mid ? a : b);
    return {
      low: Math.min(...lows),
      high: Math.max(...highs),
      mid: best.mid,
      currency: best.currency,
    };
  }

  const raw = parseRaw(str);
  if (!raw) return null;

  const currency = detectCurrency(str);
  const unit = detectUnit(str);

  // Normalise annual → monthly so all entries share a common axis
  if (unit === 'annual' && raw.mid > 0) {
    raw.low = Math.round(raw.low / 12);
    raw.high = Math.round(raw.high / 12);
    raw.mid = Math.round(raw.mid / 12);
  }

  return { ...raw, currency };
}
