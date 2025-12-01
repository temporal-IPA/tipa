# TIPA (Temporal IPA) File Format Specification

- **Version**: 1.0 (draft)
- **Author**: Benoit Pereira da Silva
- **Date**: 30/11/2025

---

## 1. Purpose and scope

TIPA (Temporal IPA) is a plain‑text format that combines:

- **International Phonetic Alphabet (IPA)** and **extIPA** transcriptions
- **Temporal anchors** (timestamps in seconds)
- **Multiple speaker / role attribution**
- **Inline annotations** (e.g. stage directions or prosodic notes)
- **Inline comments**

TIPA files are designed to be:

- Easy to read and edit by humans
- Simple to parse for tools
- Stable for serialization and round‑tripping

This document defines:

- The **syntax** of TIPA files
- The **semantics** of roles, anchors, pauses, annotations, comments, and quoted fragments
- The **“Strict TIPA” profile** (sometimes called *Strict IPA*) for normalized output

### 1.1 Non‑intrusive semantics

A central design requirement in TIPA v1.0 is:

> **Inside IPA / extIPA fragments, TIPA must stay as invisible as possible.**

Concretely:

- Phonetic material is always carried by **fragments**.  
  A fragment can be written **bare** (`bõʒuɾ ma bɛlə!`) or **double‑quoted** (`"bõʒuɾ ma bɛlə!"`).
- TIPA syntax (roles, anchors, pauses, annotations, comments, pipe delimiters) lives **around** fragments, not inside them.
- There is **no general escape character** inside fragments:
  - Backslash `\` is never a generic escape.
  - The only special sequence is `"` **inside a quoted fragment**, which stands for a literal double quote `"`.
  - All other sequences containing `\` (for example `p\p\p`) are taken **literally**.

The quoting layer is deliberately minimal: it exists only to make it possible to embed structural characters such as `[` `]` and `|` inside fragments in a controlled way.

---

## 2. File format basics

### 2.1 Encoding

- A TIPA file **must** be encoded in UTF‑8.
- IPA symbols are written using Unicode code points.
- Both **standard IPA** and **extended IPA** (extIPA, tone letters, prosodic symbols, extra diacritics, combining marks, etc.) may be used freely in fragments.
- The companion document `references.md` (if present) may enumerate the graphemes used by TIPA‑aware tools.

### 2.2 Lines and line breaks

- A TIPA document is a sequence of **lines** separated by:
  - `LF` (`\n`), or
  - `CRLF` (`\r\n`), or
  - `CR` (`\r`).
- Leading and trailing whitespace on a line is ignored **except** inside:
  - fragments (bare or quoted)
  - annotations (`[...]`)

### 2.3 Line types

Every non‑empty, non‑whitespace line is exactly one of:

1. **Comment line**
2. **Role declaration line**
3. **Utterance line** (a TIPA “sentence”)

---

## 3. Comments

### 3.1 Syntax

Comments are introduced with `#` or `##` followed by a space.

- A **whole‑line comment** is a line whose first non‑whitespace characters are:

  ```text
  # <space>...
  ## <space>...
  ```

- An **inline comment** can appear after an utterance on the same line:

  ```text
  @benoit: 156.000 | "bõʒuɾ ma bɛlə!" | 156.800  # inline comment
  @charlotte: 157.097 | "bõʒuɾ" [en souriant] | 157.600  ## stronger comment
  ```

Parsing rules:

- The first `#` that:
  - is **not** inside an annotation (`[...]`), and
  - is **not** inside a quoted fragment (`"..."`), and
  - is either the first non‑whitespace character on the line **or** is preceded by whitespace, and
  - is followed by a space (`# ` or `## `)

  starts a comment that runs to the end of the line.

- Everything from that `#` (or `##`) to the line break is ignored by parsers.

### 3.2 Pipe `|` inside fragments and annotations

The character `|` plays three different roles in TIPA:

1. As part of a **pause token** `||` between times (see §6.3)
2. As a **standalone token** used as an optional **visual delimiter** near time anchors (see §6.2.2)
3. As the standard **IPA “minor group boundary”** symbol **inside fragments**

Rules:

- `||` is always the dedicated **pause** token (silent interval).
- A standalone `|` token (surrounded by whitespace, not inside quotes or annotations) may be treated as a **delimiter** and ignored for timing.
- A `|` that appears *inside a fragment* is always phonetic content.

Quoting requirement:

- A **bare fragment** (see §5.1) **must not** contain `|`.
- Any fragment whose phonetic content contains `|` **must** be written as a **quoted fragment**:

  ```text
  3.000 | "ka|ta" | 4.000
  ```

Inside annotations:

- `|` is an ordinary character and never needs to be escaped.
- TIPA does not assign any special meaning to `|` inside annotations or inside quoted fragments.

---

## 4. Roles

TIPA supports multi‑role transcripts (e.g., different speakers in a dialogue).

### 4.1 Role names

- A role name is introduced with `@` and must not contain spaces.
- Role name characters **must not** include whitespace.
- A simple recommended pattern is:

  ```text
  @<roleId>
  ```

Examples:

```text
@benoit
@charlotte
@spk1
@Narrator
```

### 4.2 Role declarations

A role may be declared anywhere in the file using:

```text
@<roleId><spaces>=<space><role definition...>
```

- `<roleId>`: name without spaces.
- `<role definition>`: free‑text description (optional but recommended).

Example:

```text
@benoit = A man
@charlotte = A woman
```

A declaration line is any line that matches `@roleId = ...` and does **not** contain `:` immediately after the role name.

### 4.3 Utterance role prefix

An utterance line can be attributed to a role using:

```text
@<roleId>: <utterance body...>
```

Example:

```text
@benoit:   156.000 | "bõʒuɾ ma bɛlə!" | 156.800
@charlotte: 157.097 | "bõʒuɾ" [en souriant] | 157.600 158.088 | "bø" | 158.120 | "nwa" [trainant] | 159.000
```

Rules:

- Optional whitespace is allowed between the role name and the colon, and after the colon.
- The **first colon `:`** after a role name is treated as the role/utterance separator.
- Outside this role prefix, the colon `:` has no special meaning and can be used freely inside fragments or annotations.

### 4.4 Utterances without explicit role

- An utterance line **may omit** any role prefix:

  ```text
  156.000 | "bõʒuɾ ma bɛlə!" | 156.800
  ```

- For **Strict TIPA** (see §9), a linter must treat such lines as being attributed to a default role `@0` and rewrite them as:

  ```text
  @0: 156.000 | "bõʒuɾ ma bɛlə!" | 156.800
  ```

- If any utterance uses role `@0`, a role declaration line **should** be added:

  ```text
  @0 = Default role
  ```

---

## 5. IPA fragments, annotations, and characters

### 5.1 Fragments (bare vs quoted)

An **IPA fragment** is the phonetic content between anchors and annotations, e.g.:

```text
"bõʒuɾ ma bɛlə!"
"bø"
"nwa"
"p\p\p"
"ka|ta"
```

Fragments come in two surface forms:

1. **Bare fragments** – without surrounding quotes:

   ```text
   bõʒuɾ
   ma
   bɛlə!
   ```

2. **Quoted fragments** – wrapped in ASCII double quotes:

   ```text
   "bõʒuɾ ma bɛlə!"
   "p\p\p"
   "ka|ta"
   "il a dit : "bonjour""
   ```

Semantics:

- The distinction between bare and quoted fragments is **purely syntactic**:
  - They both represent a single IPA/extIPA fragment.
  - Timing rules (§6.4) treat them identically.
- **Quoted fragments are strongly recommended** for all new content.
- In **Strict TIPA** (§9), **all fragments must be quoted**.

Structural character rule:

- A **bare fragment MUST NOT contain** any of the three structural characters:

  - `[` or `]` (used for annotations)
  - `|`       (used for delimiters and pauses)

- Any fragment whose phonetic content contains one of these characters **must** be written as a quoted fragment:

  ```text
  3.000 | "ka|ta" | 4.000
  5.000 | "p[t]"  | 5.400
  ```

Allowed content:

- Fragments may contain:
  - IPA letters (consonants and vowels)
  - IPA combining diacritics
  - IPA suprasegmentals and tone letters
  - extIPA letters, diacritics, and prosodic symbols
  - Spaces
  - Optional punctuation such as `!`, `?`, `,`, `;` etc.
  - Any other Unicode characters needed for phonetic annotation, as long as the structural character rule above is respected.

### 5.2 Annotations

Annotations add arbitrary metadata and are written in square brackets:

```text
[en souriant]
[trainant]
[very fast, F0 rising]
[meta: use casual style]
```

Rules:

- Syntax: `[annotation text]`
- Annotations may appear:
  - Between words
  - Between fragments
  - Adjacent to anchors and/or `|` delimiters

Example:

```text
@charlotte: 157.097 | "bõʒuɾ" [en souriant] | 157.600
```

Semantics:

- Between 157.097 s and 157.600 s, the fragment `"bõʒuɾ"` is spoken.
- `[en souriant]` describes how it is spoken (e.g. “smiling”).

Inside annotations:

- All characters are taken literally.
- The only characters that **cannot** appear unescaped are:
  - `]` (closes the annotation)
  - A line break

There is **no escape syntax** in annotations. If you need to represent a literal `]`, you must encode it indirectly (e.g. `⟧` or spelling “right bracket”).

Interaction with quoting:

- A `[` only starts an annotation if it is **not** inside a quoted fragment.
- A `]` only ends an annotation if it was started outside any quoted fragment.

### 5.3 Backslash and double quotes

TIPA preserves the “no general escape” principle:

- Backslash `\` is **not** an escape character in general.
- It has a special meaning **only** in a quoted fragment when followed by `"`.
- All other uses of `\` (including extIPA stutter notation) are literal.

Rules in a quoted fragment `"..."`:

- The two‑character sequence `"` represents a **literal double quote** in the fragment content.

  Example:

  ```text
  "il a dit : "bonjour""
  ```

  This fragment’s phonetic/textual content is: `il a dit : "bonjour"`.

- Any `\` that is **not** immediately followed by `"` is taken literally:

  ```text
  "p\p\p"    # content: p\p\p (extIPA stutter)
  "\"       # content: a single backslash
  "\[test]"  # content: \[test]
  ```

Rules in bare fragments and annotations:

- In **bare fragments** and in annotations, `\` has **no special meaning** at all.
- Sequences like `\[`, `\]`, `\|`, `\/` are always taken literally.

---

## 6. Temporal anchors, pipes, and pauses

### 6.1 Time format

A **time anchor** is a non‑negative real value in seconds.

The textual form is:

```text
<digits> "." <digits>
```

Examples:

- `0.250`
- `10.0`
- `156.000`
- `157.097`
- `159.00`

Formally:

```ebnf
time = digit, { digit }, ".", digit, { digit } ;
```

Notes:

- The number of digits before and after the decimal point is not limited.
  - Tools may normalize (e.g. to 3 decimals) but must preserve numeric value.
- Legacy forms are **invalid** as times:
  - `.25` – must be written `0.25` or similar.
  - `10`  – must be written `10.0` or similar.
- When times are used as anchors in utterances, they must appear as **standalone tokens** separated from other tokens by whitespace.

### 6.2 Anchors and `|` delimiters

TIPA still supports the original “bare” anchor syntax.

#### 6.2.1 Bare anchor syntax (legacy / still valid)

```text
156.000 "bõʒuɾ ma bɛlə!" 156.800
```

Semantics:

- The fragment `"bõʒuɾ ma bɛlə!"` starts at 156.000 s and ends at 156.800 s.

Variants:

1. **Start and end anchors**

   ```text
   156.000 "bõʒuɾ ma bɛlə!" 156.800
   ```

2. **Start anchor only**

   ```text
   156.000 "bõʒuɾ ma bɛlə!"
   ```

3. **End anchor only**

   ```text
   "bõʒuɾ ma bɛlə!" 156.800
   ```

4. **Inline anchor inside a word sequence**

   ```text
   158.088 "bø" 158.120 "nwa"
   ```

   `158.120` marks a boundary between `"bø"` and `"nwa"`.

Bare fragments (without quotes) may be used in these patterns for backward‑compatibility, but quoted fragments are recommended.

#### 6.2.2 Pipe‑guarded syntax (optional, recommended)

For readability, you can explicitly delimit fragments with `|` between anchors and fragments:

```text
156.000 | "bõʒuɾ ma bɛlə!" | 156.800
```

General pattern:

```text
<time> | <fragment + annotations> | <time>
```

Semantics:

- Times are still anchors.
- `|` tokens in this position are **pure delimiters** and **not part of the fragment**.
- They do not change timing; they only make anchor alignment visually obvious.

Rules:

- A **pipe delimiter** is the standalone token `|` (separated by whitespace, not inside quotes).
- A `|` token that is immediately before or after a time anchor may be treated by tools as a delimiter and **ignored** when reconstructing the phonetic string.
- All other uses of `|` (including those inside fragments and annotations) are taken literally.

Examples:

```text
156.000 | "bõʒuɾ ma bɛlə!" | 156.800
@charlotte: 157.097 | "bõʒuɾ" [en souriant] | 157.600 158.088 | "bø" | 158.120 | "nwa" [trainant] | 159.000
```

In both examples, the fragments have the same timings they would have under the bare syntax.

**Spacing variants**

- In **Strict TIPA** (§9), pipes used as delimiters **must** appear with spaces on both sides: ` | `.
- Parsers may be lenient and normalize variants like:

  ```text
  156.000 |"bõʒuɾ ma bɛlə!"|156.800
  ```

  to the canonical:

  ```text
  156.000 | "bõʒuɾ ma bɛlə!" | 156.800
  ```

  but such normalization is implementation‑defined, not part of the core grammar.

### 6.3 Pauses

A **pause** is a silent interval between two anchors and is written using `||`:

```text
<tStart> <optional whitespace> || <optional whitespace> <tEnd>
```

Example:

```text
10.300 || 10.800
```

Means: a **silent pause** between `10.300` s and `10.800` s.

Notes:

- The duration of the pause is `tEnd - tStart`.
- `||` is a dedicated pause token and is not confused with two pipe delimiters.
- As with anchors, `tStart` and `tEnd` must be valid times (`<digits>.<digits>`).

### 6.4 General anchor sequences in an utterance

An utterance body is effectively a **sequence of time anchors, pauses, optional `|` delimiters, fragments, and annotations**.

Example:

```text
@charlotte: 157.097 | "bõʒuɾ" [en souriant] | 157.600 158.088 | "bø" | 158.120 | "nwa" [trainant] | 159.000
```

Conceptually:

- Anchor `157.097`
- Fragment `"bõʒuɾ" [en souriant]`
- Anchor `157.600`
- Anchor `158.088`
- Fragment `"bø"`
- Anchor `158.120`
- Fragment `"nwa" [trainant]`
- Anchor `159.000`

Pipes `|` serve only as optional visual boundaries.

Semantics:

1. **Fragments and their anchors**

   For each fragment:

   - **Start anchor**: the closest time anchor **immediately before** the fragment on the same line.
   - **End anchor**: the closest time anchor **immediately after** the fragment on the same line.

   Ignoring any `|` delimiters and annotations in between.

   - If both exist, the fragment’s duration is defined.
   - If only one exists, only start or end time is known.
   - If no anchors exist on the line, the fragment is untimed.

2. **Shared anchors**

   A time anchor can serve as:

   - End time of the fragment before it, and
   - Start time of the fragment after it.

   In the example:

   - `"bø"` is between 158.088 s and 158.120 s.
   - `"nwa" [trainant]` is between 158.120 s and 159.000 s.

3. **Pauses**

   A pause is written as:

   ```text
   tStart || tEnd
   ```

   - It defines a silent interval.
   - It does not contain any fragment.
   - `tStart`/`tEnd` may equal adjacent fragment anchors.

### 6.5 Anchor monotonicity (per sentence)

For **Strict TIPA**:

- Within a single utterance line, collect all times that appear:
  - As standalone anchors, and
  - As left/right endpoints of pauses (`t1 || t2`).

For any consecutive pair of anchors `(t_i, t_{i+1})` used to delimit a fragment or a pause, the linter must ensure:

```text
t_{i+1} > t_i
```

- If `t_{i+1} ≤ t_i`, the utterance is invalid in Strict TIPA.

---

## 7. Utterance line structure

### 7.1 Syntax

An utterance line has the structure:

```text
[<whitespace>][@roleId: ]<utterance body>[<whitespace>][# or ## inline comment]
```

Where:

- `@roleId: ` is optional (see §4.4).
- `<utterance body>` is a sequence of:
  - time anchors (`<digits>.<digits>`)
  - pauses (`t1 || t2`)
  - optional pipe delimiters (`|`)
  - fragments (bare or quoted)
  - annotations (`[...]`)

The first `# ` or `## ` that satisfies the comment rules in §3.1 starts an inline comment to the end of the line.

### 7.2 Multiple spans per line

An utterance may contain multiple anchored regions and pauses:

```text
@charlotte: 157.097 | "bõʒuɾ" [en souriant] | 157.600 158.088 | "bø" | 158.120 | "nwa" [trainant] | 159.000
```

- Parsers should treat the entire body as a single temporal sequence.
- Whitespace outside fragments and annotations is a separator and may be normalized.
- Optional `|` delimiters do not affect timings; they are purely structural hints.

### 7.3 Utterances without anchors

Anchors are optional:

```text
@benoit: "bõʒuɾ ma bɛlə!"
```

- Such utterances are valid but have **no temporal information**.
- In Strict TIPA, these lines are allowed; the monotonicity constraint is vacuously true (no anchors).

---

## 8. Grammar (informative EBNF)

The following grammar is **informative**. It captures the intended structure but does not prescribe a particular tokenizer.

```ebnf
document        = { line }, EOF ;

line            = [ whitespace ],
                  ( comment-line
                  | role-decl-line
                  | utterance-line
                  ),
                  line-break ;

comment-line    = whitespace*, "#", [ "#" ], " ",
                  { any-char-except-line-break } ;

role-decl-line  = role-name, whitespace*, "=", whitespace*,
                  [ role-def-text ] ;

utterance-line  = [ role-prefix ],
                  utterance-body,
                  [ whitespace, inline-comment ] ;

role-prefix     = role-name, whitespace*, ":", whitespace* ;

role-name       = "@", role-id ;
role-id         = 1*( nonspace-char - ":" - "=" ) ;

utterance-body  = { utterance-token, [ whitespace ] } ;

utterance-token = pause
                | time-anchor
                | pipe-delimiter
                | annotation
                | fragment ;

pause           = time, whitespace*, "||", whitespace*, time ;

time-anchor     = time ;

pipe-delimiter  = "|" ;

annotation      = "[", { annotation-char }, "]" ;

fragment        = bare-fragment
                | quoted-fragment ;

bare-fragment   = { bare-fragment-char }+ ;

(* Bare fragments cannot contain square brackets, pipe or double quote *)
bare-fragment-char
                = unicode-char
                  - line-break
                  - "["
                  - "]"
                  - "|"
                  - '"' ;

quoted-fragment = '"', { quoted-char }, '"' ;

(* Inside a quoted fragment, all characters except line break and
   unescaped double quote are allowed. The two-character sequence " is
   interpreted as a literal double quote in the fragment content. *)
quoted-char     = unicode-char
                  - line-break
                  - '"' ;

annotation-char = unicode-char
                  - line-break
                  - "["    (* nested [ not allowed *)
                  - "]" ;  (* closes the annotation *)

time            = digit, { digit }, ".", digit, { digit } ;

inline-comment  = "#", [ "#" ], " ",
                  { any-char-except-line-break } ;
```

**Notes for implementers**

- The grammar treats `|` as a generic `pipe-delimiter` token when it appears outside fragments and annotations.
  - In lines with **no times**, these tokens can be treated as 1‑character IPA fragments `|` (IPA minor group boundary) if desired.
  - In lines with times:
    - `|` tokens directly adjacent to times can be treated as visual delimiters and ignored for timing.
    - A literal `|` needed *inside* a fragment must be written in a quoted fragment (§5.1).
- There is **no general `escape-seq` production**:
  - Backslash `\` has special meaning only as part of `"` inside a quoted fragment.
  - All other uses of `\` are literal.

---

## 9. Strict TIPA profile (“Strict IPA”)

A **Strict TIPA** document is a normalized form for tooling and storage.

A linter that transforms a TIPA file into Strict TIPA must apply the following rules:

### 9.1 Role injection

For every utterance line that does **not** start with `@roleId:` or `@roleId =`:

1. Prepend the default role identifier `@0: `.
2. The content of the line after injection must be unchanged (except for normalization of surrounding whitespace).

Example input:

```text
156.000 | "bõʒuɾ ma bɛlə!" | 156.800
```

Strict TIPA output:

```text
@0: 156.000 | "bõʒuɾ ma bɛlə!" | 156.800
```

### 9.2 Role declarations

For each role used in at least one utterance (i.e., appearing as `@roleId:`):

- Ensure there is at least one **role declaration line** of the form:

  ```text
  @roleId = <description>
  ```

- If a role is used but not declared, the linter must insert a declaration such as:

  ```text
  @benoit =
  @charlotte =
  @0 = Default role
  ```

- The exact placement is implementation‑defined, but a common convention is to insert all missing declarations at the top of the file, before the first utterance.

### 9.3 Anchor consistency per sentence

For each utterance line:

1. Collect all time values that appear:
   - As standalone anchors (tokens matching `time` in the utterance body).
   - As left/right endpoints of pauses (`t1 || t2`).

2. Sort them in order of appearance on the line: `t0, t1, t2, ...`.

For every pair of consecutive times `(t_i, t_{i+1})` that delimit a fragment or a pause, enforce:

```text
t_{i+1} > t_i
```

- If this condition is violated, the linter should report an error or warning; producing Strict TIPA output is not possible without changing timing.

### 9.4 Anchor optionality

- Zero anchors: an utterance with no anchors is valid.
- Single anchor: an utterance with a single anchor is valid; only one boundary is known.
- Multiple anchors: must satisfy the monotonicity rule above.

### 9.5 Structural character and quoting invariants

In Strict TIPA:

- **All fragments must be quoted**.  
  Bare fragments are accepted only in non‑strict input; a Strict TIPA linter must rewrite them as quoted fragments.

  Example:

  ```text
  @benoit: 156.000 | bõʒuɾ ma bɛlə! | 156.800
  ```

  becomes:

  ```text
  @benoit: 156.000 | "bõʒuɾ ma bɛlə!" | 156.800
  ```

- Inside fragment text:
  - `[` and `]` have no special meaning and may appear freely **only inside quoted fragments**.
  - `|` used as the IPA minor group boundary must appear inside quoted fragments.
- There are **no escape sequences** other than `"` inside a quoted fragment:
  - Backslash `\` appears only as itself (typically extIPA stutter) except when forming `"`.
  - Sequences such as `\[`, `\]`, `\`, `\|`, `\/` are treated literally.
- Pipes used as anchor delimiters must be in canonical form:

  ```text
  space, "|", space
  ```

  For example:

  ```text
  156.000 | "bõʒuɾ ma bɛlə!" | 156.800
  ```

- Whitespace between tokens:
  - Outside fragments and annotations, whitespace may be normalized to single spaces.
  - Inside fragments and annotations, whitespace must be preserved verbatim.

---

## 10. Examples

### 10.1 Simple dialogue with roles, timing, and pipes

```text
@hamlet = Prince of Denmark
@ophelia = Daughter of Polonius

# Soliloquy
@hamlet:   156.000 | "tə ˈbiː ɔː ˈnɒt tə ˈbiː" | 158.000
@ophelia:  158.500 | "ðæt ɪz ðə ˈkwestʃən" [aside] | 160.000
```

### 10.2 Example with a pause

```text
@0 = Default role

@0: 10.000 | "ɔːl" | 10.300 10.300 || 10.800 10.800 | "ðə wɜːldz ə steɪdʒ" | 11.600
```

Semantics:

- `"ɔːl"` from 10.000 s to 10.300 s
- Silence from 10.300 s to 10.800 s
- `"ðə wɜːldz ə steɪdʒ"` (“the world's a stage”) from 10.800 s to 11.600 s

### 10.3 IPA/extIPA inside fragments (no escaping except `"`)

```text
@spk1 = Narrator

@spk1: 0.000 | "hiː sɛz /tə ˈbiː ɔː ˈnɒt tə ˈbiː/" | 2.500
@spk1: 2.500 | "p\p\p" [stuttered onset] | 3.000
@spk1: 3.000 | "ka|ta" [minor group boundary in IPA] | 4.000
@spk1: 4.000 | "hiː sɛd : "ðə pleɪz ðə θɪŋ"" | 5.000
```

Interpretation:

- The first fragment runs from 0.000 s to 2.500 s and contains `/.../` literally (phonemic slashes around the *“to be, or not to be”* IPA string).
- The second fragment runs from 2.500 s to 3.000 s and contains `p\p\p`; the backslash is purely extIPA (stutter notation).
- The third fragment uses `|` inside the IPA string as a minor group boundary, enabled by quoting.
- The fourth fragment shows how to embed a literal double quote using `"` around *“the play's the thing”*.

---

## 11. Summary

- TIPA is a UTF‑8 text format for IPA/extIPA transcripts with:
  - Multi‑role attribution using `@roleId` and `@roleId:`
  - Temporal anchors expressed as real values in seconds, written as `<digits>.<digits>` tokens
  - Pauses with `t1 || t2`
  - Optional pipe delimiters `|` around anchored fragments
  - Inline annotations in `[ ... ]`
  - Inline and whole‑line comments using `# ` and `## `
  - **Optional double‑quoted fragments**, which are **required** in Strict TIPA

- TIPA v1.0 guarantees that:
  - Inside fragments, IPA/extIPA graphemes (including `\` and `|`) are taken literally.
  - The only special sequence inside a fragment is `"` in a quoted fragment.
  - Square brackets used for annotations and pipe delimiters around anchors are never confused with fragment content.

- The **Strict TIPA** profile defines a canonicalized form suitable for tooling and storage, enforcing:
  - Explicit roles for every utterance (`@0:` when none is given)
  - Role declarations for all used roles
  - Monotonic anchor times within each utterance
  - All fragments written as quoted strings
  - Structural use of `|` in the canonical ` | ` form
  - Clean separation between annotations (`[...]`) and phonetic content.

© 2025 Benoit Pereira da Silva – Licensed under Creative Commons Attribution 4.0 International (CC BY 4.0).
