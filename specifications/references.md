# IPA & ExtIPA Grapheme Manual
_Companion to the TIPA format – Version 1.0_

This document enumerates the Unicode graphemes used by:

- the **International Phonetic Alphabet (IPA)** – consonant and vowel letters, diacritics, suprasegmentals and tone marks
- the **Extensions to the IPA for Disordered Speech (extIPA)** – additional letters, diacritics and prosodic symbols

It is designed as a practical, single-file reference for tools such as **TIPA**.  
The focus is on *which symbols exist* and how they are grouped, not on phonetic theory.

---

## 1. Reading this manual

### 1.1 Notation

- **Letters** are written in back‑ticks, e.g. `p`, `ɲ`, `ʃ`.
- **Combining diacritics** are shown with a dotted circle: `◌̪`, `◌̥`, `◌̃`, etc.
- **Suprasegmentals and tone letters** are spacing symbols like `ˈ`, `ː`, `˥`.
- Many symbols come from the **IPA Extensions** and **Latin Extended** Unicode blocks; font support may vary.

### 1.2 High level categories

For TIPA and similar tools it is often useful to keep these categories separate:

1. **IPA base letters**
2. **IPA combining diacritics**
3. **IPA suprasegmentals and tone symbols**
4. **extIPA additional letters**
5. **extIPA diacritics and prosodic / indeterminate markers**
6. **Legacy & compatibility symbols** (rare or deprecated, but still encountered)

---

## 2. Core IPA base letters

### 2.1 Pulmonic consonant letters

These are the consonant letters that appear in the main IPA pulmonic chart.

#### Plosives

- Bilabial: `p`, `b`
- Alveolar: `t`, `d`
- Retroflex: `ʈ`, `ɖ`
- Palatal: `c`, `ɟ`
- Velar: `k`, `ɡ`
- Uvular: `q`, `ɢ`
- Glottal: `ʔ`

#### Nasals

- Bilabial: `m`
- Labiodental: `ɱ`
- (Dental / alveolar): `n`
- Retroflex: `ɳ`
- Palatal: `ɲ`
- Velar: `ŋ`
- Uvular: `ɴ`

#### Trills

- Bilabial: `ʙ`
- Alveolar: `r`
- Uvular: `ʀ`

#### Taps / flaps

- Labiodental: `ⱱ`
- Alveolar: `ɾ`
- Retroflex: `ɽ`

#### Fricatives (central)

- Bilabial: `ɸ`, `β`
- Labiodental: `f`, `v`
- Dental: `θ`, `ð`
- Alveolar: `s`, `z`
- Postalveolar: `ʃ`, `ʒ`
- Retroflex: `ʂ`, `ʐ`
- Palatal: `ç`, `ʝ`
- Velar: `x`, `ɣ`
- Uvular: `χ`, `ʁ`
- Pharyngeal: `ħ`, `ʕ`
- Glottal: `h`, `ɦ`

#### Lateral fricatives

- Alveolar: `ɬ`, `ɮ`

#### Approximants (central)

- Labiodental: `ʋ`
- Alveolar: `ɹ`
- Retroflex: `ɻ`
- Palatal: `j`
- Velar: `ɰ`

#### Lateral approximants

- Alveolar: `l`
- Retroflex: `ɭ`
- Palatal: `ʎ`
- Velar: `ʟ`

### 2.2 “Other symbols” for consonants

These are IPA letters that do not sit in the main pulmonic table but are part of the standard.

- Co‑articulated approximants and fricatives:
  - `ʍ` – voiceless labial‑velar fricative/approximant
  - `w` – labial‑velar approximant
  - `ɥ` – labial‑palatal approximant
  - `ɧ` – so‑called “sj‑sound” (simultaneous palatal‑velar fricative)
- Epiglottal / pharyngeal:
  - `ʜ` – voiceless epiglottal fricative
  - `ʢ` – voiced epiglottal fricative
  - `ʡ` – epiglottal stop
- Alveolo‑palatal:
  - `ɕ` – voiceless alveolo‑palatal fricative
  - `ʑ` – voiced alveolo‑palatal fricative
- Other:
  - `ɺ` – alveolar lateral flap

(These letters are all standard IPA; many are also referenced in extIPA.)

### 2.3 Non‑pulmonic consonant letters

#### Clicks

- `ʘ` – bilabial click  
- `ǀ` – dental click  
- `ǃ` – (post)alveolar click  
- `ǂ` – palato‑alveolar click  
- `ǁ` – alveolar lateral click  

#### Implosives

- `ɓ` – bilabial implosive  
- `ɗ` – dental/alveolar implosive  
- `ʄ` – palatal implosive  
- `ɠ` – velar implosive  
- `ʛ` – uvular implosive  

#### Ejectives

- `ʼ` – ejective sign (used after a consonant: `tʼ`, `kʼ`, etc.)

### 2.4 Vowel letters

The following are the vowel letters in the current IPA vowel chart.

#### Close (high) vowels

- Front: `i`, `y`
- Central: `ɨ`, `ʉ`
- Back: `ɯ`, `u`

#### Near‑close vowels

- Front: `ɪ`, `ʏ`
- Back: `ʊ`

#### Close‑mid vowels

- Front: `e`, `ø`
- Central: `ɘ`, `ɵ`
- Back: `ɤ`, `o`

#### Mid central vowel

- `ə` – mid central (schwa)

#### Open‑mid vowels

- Front: `ɛ`, `œ`
- Central: `ɜ`, `ɞ`
- Back: `ʌ`, `ɔ`

#### Near‑open vowels

- Front: `æ`
- Central: `ɐ`

#### Open (low) vowels

- Front: `a`, `ɶ`
- Back: `ɑ`, `ɒ`

### 2.5 Legacy / compatibility vowel letters

These letters are widely used but can be analysed as a base vowel + diacritic:

- `ɚ` – rhotic schwa (`ə˞`)
- `ɝ` – rhotic open‑mid central (`ɜ˞`)

Tools MAY choose to normalise them internally to base vowel + rhoticity diacritic (see §3.4).

---

## 3. IPA combining diacritics

This section lists **combining marks** that the IPA treats as diacritics.  
They normally follow the base symbol and combine above, below, through, or after it.

### 3.1 Voicing & phonation

- `◌̥` – voiceless (small ring below)
- `◌̊` – voiceless (ring above; often used on vowels)
- `◌̬` – voiced (voicing mark)
- `◌̤` – breathy voiced (murmured; diaeresis below)
- `◌̰` – creaky voiced (tilde below)

### 3.2 Nasality & rhoticity

- `◌̃` – nasalized
- `◌̘` – advanced tongue root (ATR)
- `◌̙` – retracted tongue root (RTR)
- `◌˞` – rhoticity (right hook; e.g. `ə˞`, `a˞`)

### 3.3 Place & secondary articulation

- `◌̪` – dental (subscript bridge)
- `◌̺` – apical (inverted bridge below)
- `◌̻` – laminal / blade (square below)
- `◌̟` – advanced (tongue moved forward)
- `◌̠` – retracted (tongue moved back)
- `◌̝` – raised (closer / more constricted)
- `◌̞` – lowered (more open / less constricted)
- `◌̹` – more rounded (right half‑ring)
- `◌̜` – less rounded (left half‑ring)
- `◌̴` – velarized or pharyngealized (tilde through middle)
- `◌̽` – mid‑centralized (X above)
- `◌̈` – centralized

### 3.4 Syllabicity and glides

- `◌̩` – syllabic
- `◌̯` – non‑syllabic (glide)
- `◌̑` (older usage) – moric / length mark in some traditions (rare)

### 3.5 Airstream & release

- `ʰ` – aspirated (superscript h after the consonant)
- `◌˭` – unaspirated (used mainly in extIPA; see also §5.2)
- `ⁿ` – nasal release (superscript n)
- `ˡ` – lateral release (superscript l)
- `◌̚` – no audible release
- `ʷ` – labialized (superscript w)
- `ʲ` – palatalized (superscript j)
- `ˠ` – velarized (superscript gamma)
- `ˤ` – pharyngealized (superscript reversed glottal stop)

### 3.6 Linguolabial and related

- `◌̼` – linguolabial (tongue against upper lip)

---

## 4. IPA suprasegmentals and boundaries

These are spacing symbols that affect stress, length, grouping or linking.

### 4.1 Stress & prominence

- `ˈ` – primary stress (before the stressed syllable)
- `ˌ` – secondary stress

### 4.2 Length & timing

- `ː` – long (length mark)
- `ˑ` – half‑long
- `◌̆` – extra‑short (combining breve)

### 4.3 Grouping & breaks

- `.` – syllable break
- `|` – minor (foot) group boundary
- `‖` – major (intonation) group boundary
- `‿` – linking / absence of break (undertie)
- `͡` – combining tie bar above (affricates, double articulations)
- `͜` – combining tie bar below (affricates, double articulations)

---

## 5. IPA tone and intonation symbols

Two equivalent families of tone notation are widely used.

### 5.1 Level & contour marks over a vowel

Placed above the vowel (combining):

- Level tones:
  - `◌̋` – extra‑high
  - `◌́` – high
  - `◌̄` – mid
  - `◌̀` – low
  - `◌̏` – extra‑low
- Simple contours:
  - `◌̌` – rising
  - `◌̂` – falling
  - `◌᷄` – high rising (mid + high)
  - `◌᷅` – low rising
  - `◌᷇` – rising‑falling (etc., rarely needed in most work)

### 5.2 Chao tone letters (vertical bars)

Placed after the syllable nucleus, representing pitch on a 5‑point scale:

- Level: `˥` (55 extra high), `˦` (44 high), `˧` (33 mid), `˨` (22 low), `˩` (11 extra‑low)
- Contours by sequence:  
  - `˧˥` – rising (35)  
  - `˨˩` – falling (21)  
  - Longer sequences such as `˩˧˥` for complex contours.

---

## 6. extIPA additional letters

The extIPA adds letters for sounds not covered by base IPA, especially in disordered speech.  
They are grouped here by rough articulatory class.

### 6.1 Lateral + median sibilants

- `ʪ` – voiceless grooved lateral alveolar fricative (laterally lisped /s/)
- `ʫ` – voiced grooved lateral alveolar fricative (laterally lisped /z/)

### 6.2 Lateral fricatives (implicit in IPA, explicit in extIPA)

- `ꞎ` – voiceless retroflex lateral fricative
- `𝼅` – voiced retroflex lateral fricative
- `𝼆` – voiceless palatal lateral fricative
- `𝼄` – voiceless velar lateral fricative

(Voiced palatal/velar lateral fricatives are often written as `𝼆̬`, `𝼄̬` or with `ʎ̝`, `ʟ̝` in pure IPA.)

### 6.3 Velopharyngeal series

- `ʩ` – voiceless velopharyngeal fricative  
- `ʩ̬` – voiced velopharyngeal fricative (letter + `◌̬`)
- `𝼀` – voiceless velopharyngeal trill (“snort”; often roughly `[ʩ` + uvular trill] )

### 6.4 Velodorsal series

- `𝼃` – voiceless velodorsal plosive
- `𝼁` – voiced velodorsal plosive
- `𝼇` – velodorsal nasal

### 6.5 Upper‑pharyngeal plosives

- `ꞯ` – voiceless upper‑pharyngeal plosive
- `𝼂` – voiced upper‑pharyngeal plosive

### 6.6 Percussive consonants

- `ʬ` – bilabial percussive (lips smacking together)
- `ʭ` – bidental percussive (gnashing teeth)
- `¡` – sublaminal lower‑alveolar percussive (tongue slap), used also in click releases (`ǃ¡`, `ǂ¡`)

---

## 7. extIPA diacritics (extensions to IPA diacritics)

The extIPA reuses all ordinary IPA diacritics (see §3), and **adds** or **specialises** several more.

### 7.1 Airstream & airflow

Placed after a segment unless noted:

- `↓` – ingressive airflow (after a segment: `p↓`)
- `↑` – egressive airflow (after a segment: `p↑`) – now less often used on the chart, but still attested
- Isolated arrows:
  - `↓` alone – inhalation noise
  - `↑` alone – exhalation noise

### 7.2 Phonation & aspiration refinements

- `˭` – unaspirated plosive (e.g. `p˭`)
- `ʰp` – pre‑aspiration (aspiration before closure rather than after release)
- Extended timing of creak/voicelessness etc. with modifier letters such as:
  - `ˬ` – pre‑ or post‑voicing when placed before/after a segment
  - `˷` – creaky off‑glide on a vowel
  - `˳` – extended voicelessness after a segment

(Any IPA phonation diacritic may also be displaced to indicate timing relative to the segment.)

### 7.3 Nasal frication & denasalization

- `◌̾` on a **nasal** – nareal (nasal) fricative (noise at the nostrils)
- Special velopharyngeal friction marker (Unicode U+10790, rendered here as `◌` on oral or nasal letters) – velopharyngeal friction
- `◌̾` on an **oral** segment (e.g. `v̾`) – nasal fricative escape (audible nasal turbulence)
- `◌͊` – (partially) denasalized (e.g. `m͊` for denasal /m/)

### 7.4 Strength of articulation

- `◌͈` – strong articulation (very tense or forceful constriction)
- `◌͉` – weak articulation (reduced constriction)

### 7.5 Fine place & shape details

These refine the information given by ordinary IPA diacritics:

- `◌͆` – dentolabial or class‑3 occlusion depending on context  
  - On labials (e.g. `v͆`) – dentolabial (lower lip against upper teeth)  
  - On coronals with `◌̪` – interdental / bidental
  - On `h` – bidental fricative (teeth closely opposed)
- `◌͇` – explicit alveolar articulation on coronals (used to contrast with dental / laminal etc.)
- `◌͍` – labial spreading (e.g. `s͍`, `u͍`)
- `◌͎` – whistled articulation (e.g. `s͎`)
- `s̻`, `z̻` – laminal sibilants (blade of tongue active)
- `s͔`, `s͕` – main gesture offset right / left (used for complex tongue shapes)
- Many of these are used especially in clinical descriptions where fine tongue posture is important.

### 7.6 Timing & complex gestures

- `◌͢◌` – sliding (slurred) articulation between two consonants: `s͢θ`, `x͢ɕ`
- `p\p\p` – stutter / reiterated articulation (backslash as repetition marker)
- `(◌)` – diacritics in parentheses indicate **partial** application in degree or in time
  - `s̬᪽` – partial voicing
  - `s̬᫃` – voicing at beginning only
  - `s̬᫄` – voicing at end only  
  (Analogous forms exist for devoicing and for other phonation types.)

### 7.7 Rhythm, uncertainty & indeterminate segments

- `◯` – indeterminate segment
- `◯σ` – indeterminate syllable
- `Ⓒ` – some indeterminate consonant
- `Ⓥ` – some indeterminate vowel
- `Ⓕ`, `ⓟ`, etc. – indeterminate fricative, indeterminate plosive, etc.
- Circled IPA letters (e.g. `ⓚ`) – “probably this sound”, identification uncertain
- `( … )` – mouthing / silent articulation or a silent pause; the duration may be written inside: `(2.3 s)`
- `⸨ … ⸩` – region obscured by noise or overlapping speech (double parentheses style brackets)

---

## 8. Legacy and rarely used symbols

These symbols are rarely used in modern IPA/extIPA transcriptions, but may appear in older sources or specialised literature. Tools may choose to support them for completeness.

- Rhotic vowels as single letters: `ɚ`, `ɝ` (see §2.5)
- Old “implicit” retroflex series used in Unicode proposals: `ᶑ`, `𝼈`, etc.
- Palatal hooks and other historical diacritics now replaced by superscript `ʲ` and related marks.

For detailed historical coverage, consult the IPA Handbook and the most recent IPA & extIPA charts.

---

## 9. Flat symbol inventories (for implementers)

This section gives **flat lists** of graphemes that are typically useful when implementing parsers, lexers, or linters.

### 9.1 IPA base letters (consonants + vowels)

```text
p b t d ʈ ɖ c ɟ k ɡ q ɢ ʔ
m ɱ n ɳ ɲ ŋ ɴ
ʙ r ʀ
ⱱ ɾ ɽ
ɸ β f v θ ð s z ʃ ʒ ʂ ʐ ç ʝ x ɣ χ ʁ ħ ʕ h ɦ
ɬ ɮ
ʋ ɹ ɻ j ɰ
l ɭ ʎ ʟ
ʍ w ɥ ʜ ʢ ʡ ɕ ʑ ɺ ɧ
ʘ ǀ ǃ ǂ ǁ
ɓ ɗ ʄ ɠ ʛ
i y ɨ ʉ ɯ u
ɪ ʏ ʊ
e ø ɘ ɵ ɤ o
ə
ɛ œ ɜ ɞ ʌ ɔ
æ ɐ
a ɶ ɑ ɒ
```

(Optional legacy: `ɚ ɝ`)

### 9.2 IPA combining diacritics (core set)

```text
◌̥ ◌̊ ◌̬ ◌̤ ◌̰
◌̃ ◌̘ ◌̙ ◌˞
◌̪ ◌̺ ◌̻ ◌̟ ◌̠ ◌̝ ◌̞ ◌̹ ◌̜
◌̴ ◌̽ ◌̈
◌̩ ◌̯
ʰ ◌˭ ⁿ ˡ ◌̚
ʷ ʲ ˠ ˤ
◌̼
```

(Plus the tone‑mark diacritics:)

```text
◌̋ ◌́ ◌̄ ◌̀ ◌̏
◌̌ ◌̂ ◌᷄ ◌᷅ ◌᷇
```

### 9.3 IPA suprasegmentals & tone letters (spacing)

```text
ˈ ˌ
ː ˑ ◌̆
. | ‖ ‿ ͡ ͜
˥ ˦ ˧ ˨ ˩
```

### 9.4 extIPA additional letters

```text
ʪ ʫ
ꞎ 𝼅 𝼆 𝼄
ʩ 𝼀
𝼃 𝼁 𝼇
ꞯ 𝼂
ʬ ʭ ¡
```

### 9.5 extIPA‑specific diacritics and symbols (high‑value subset)

```text
↓ ↑
˭ ˬ ˷ ˳
◌̾ ◌͊
◌͈ ◌͉
◌͆ ◌͇ ◌͍ ◌͎
◌͢
\  (backslash used as stutter marker in sequences like p\p\p)
◯ Ⓒ Ⓥ Ⓕ ⓟ
( ) ⸨ ⸩
```

(Plus: parenthesised or displaced versions of ordinary IPA diacritics as described in §7.6.)

---

This completes the grapheme inventory for IPA and extIPA as used by TIPA.  
For definitive phonetic definitions and any future updates, always defer to:

- the official IPA chart published by the **International Phonetic Association**
- the current **extIPA chart for disordered speech** published by **ICPLA**.


© 2025 Benoit Pereira da Silva – Licensed under Creative Commons Attribution 4.0 International (CC BY 4.0).

