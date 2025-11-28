# TIPA (Temporal IPA) File Format Specification

- **Version**: 1.0 (draft)
- **Author**: Benoit Pereira da Silva 
- **Date**: 28/11/2025

---

## 1. Purpose and scope

TIPA (Temporal IPA) is a plain-text format that combines:

- **International Phonetic Alphabet (IPA)** transcriptions
- **Temporal anchors** (timestamps in seconds)
- **Multiple speaker / role attribution**
- **Inline annotations** (e.g. stage directions or prosodic marks)
- **Inline comments**

TIPA files are designed to be:

- Easy to read and edit by humans
- Simple to parse for tools
- Stable for serialization and round‑tripping

This document defines:

- The **syntax** of TIPA files
- The **semantics** of roles, anchors, pauses, annotations, and escaping
- The **“Strict TIPA” profile** (called *Strict IPA* in some tools) for normalized output

---

## 2. File format basics

### 2.1 Encoding

- A TIPA file **must** be encoded in UTF‑8.
- IPA symbols are written using Unicode code points.
- Both **standard IPA** and **extended IPA** (extIPA, tone letters, prosodic symbols, extra diacritics, combining marks, etc.) may be used freely.  
  They behave like any other IPA glyph and **do not require special escaping** unless they are literally the characters `[`, `]`, or `\`.

### 2.2 Lines and line breaks

- A TIPA document is a sequence of **lines** separated by `LF` (`\n`), `CRLF` (`\r\n`), or `CR` (`\r`).
- Leading and trailing whitespace on a line is ignored **except** inside:
  - IPA fragments
  - Annotations (`[...]`)

### 2.3 Line types

Every non-empty, non-whitespace line is exactly one of:

1. **Comment line**
2. **Role declaration line**
3. **Utterance line** (a TIPA “sentence”)

---

## 3. Comments

### 3.1 Syntax

Comments are introduced with `#` or `##` followed by a space.

- A **whole-line comment** is a line whose first non-whitespace characters are:

  ```text
  # <space>...
  ## <space>...
  ```

- An **inline comment** can appear after an utterance on the same line:

  ```text
  @benoit: 156.000 bõʒuɾ ma bɛlə! 156.800  # inline comment
  @charlotte: 157.097 bõʒuɾ [en souriant] 157.600  ## stronger comment
  ```

Parsing rules:

- The first `#` that:
  - is **not** inside an annotation (`[...]`), and
  - is either the first non-whitespace character on the line, **or** is preceded by whitespace,
  - and is followed by a space (`# ` or `## `)

  starts a comment that runs to the end of the line.

- Everything from that `#` (or `##`) to the line break is ignored by parsers.

### 3.2 Slash `/` and pipe `|` inside IPA and annotations

- The `/` and `|` characters **never** start comments in TIPA.
- They have **no special meaning** in IPA fragments or annotations and **do not need to be escaped**.
- Inside IPA and annotations, `/` and `|` are ordinary characters, just like any other IPA or Unicode symbol.

---

## 4. Roles

TIPA supports multi‑role transcripts (e.g., speakers in a dialogue).

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
- `<role definition>`: free text description (optional but recommended).

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
@benoit: 156.000 bõʒuɾ ma bɛlə! 156.800
@charlotte: 157.097 bõʒuɾ [en souriant] 157.600 158.088 bø 158.120 nwa[trainant] 159.00
```

Rules:

- Optional whitespace is allowed between the role name and the colon, and after the colon.
- The **first colon `:`** after a role name is treated as the role/utterance separator.
- Outside this role prefix, the colon `:` has no special meaning and can be used freely inside IPA fragments and annotations.

### 4.4 Utterances without explicit role

- An utterance line **may omit** any role prefix:

```text
156.000 bõʒuɾ ma bɛlə! 156.800
```

- For **Strict TIPA** (see §9), a linter must treat such lines as being attributed to a default role `@0` and rewrite them as:

```text
@0: 156.000 bõʒuɾ ma bɛlə! 156.800
```

- If any utterance uses role `@0`, a role declaration line **should** be added:

```text
@0 = Default role
```

---

## 5. IPA fragments, annotations, and escaping

### 5.1 IPA fragments

An **IPA fragment** is the phonetic content between anchors and annotations, e.g.:

```text
bõʒuɾ ma bɛlə!
bø
nwa
```

- IPA fragments may contain:
  - IPA letters and diacritics
  - Extended IPA (extIPA) symbols
  - Combining marks, tone letters, arrows, prosodic symbols, etc.
  - Spaces
  - Other Unicode symbols that are not structural characters.

### 5.2 Annotations

Annotations add arbitrary metadata inside an utterance and are written in square brackets:

```text
[en souriant]
[trainant]
[very fast, F0 rising]
```

Rules:

- Syntax: `[annotation text]`
- Annotations may appear:
  - Between words
  - Between IPA fragments
  - Adjacent to anchors

Example:

```text
@charlotte: 157.097 bõʒuɾ [en souriant] 157.600
```

This means:

- Between 157.097 s and 157.600 s, the IPA fragment `bõʒuɾ` is spoken.
- The annotation `[en souriant]` describes how it is spoken (e.g., “smiling”).

### 5.3 Escaping special characters

The TIPA escape character is the backslash `\`.

Inside **IPA fragments** and **annotations**, escaping is intentionally minimal:

- The **only characters that need escaping** are:
  - `[` and `]`  (annotation delimiters)
  - `\`          (the escape character itself)

All other characters – including `/`, `|`, `:`, `@`, `#`, and any standard or extended IPA symbol – are taken **literally** inside IPA fragments and annotations.

#### 5.3.1 Escaping in IPA fragments

Inside IPA fragments:

- `\[` → literal `[`
- `\]` → literal `]`
- `\\` → literal `\`

Examples:

```text
bɛlə \[emphasis\]          # literal [emphasis]
pa:se                      # colon is literal, no escaping
tʃaɪ̈/ʃɜːt|test            # / and | are literal IPA/text
```

#### 5.3.2 Escaping in annotations

Inside annotations `[ ... ]`:

- The annotation text can contain **any characters** except:
  - an unescaped `[` or `]` (which would conflict with delimiters), and
  - a bare line break.

To write literal brackets or backslashes:

- `\[` → literal `[`
- `\]` → literal `]`
- `\\` → literal `\`

Example:

```text
[prosody: \[focus\] on last syllable]
[note: literal \[ and \] and / and | are all fine]
```

Extended IPA characters and any other Unicode symbols (tone letters, arrows, etc.) can be used **directly** in annotations without escaping.

---

## 6. Temporal anchors and pauses

### 6.1 Time format

A **time anchor** is a non‑negative real value in seconds.

Allowed textual forms:

```text
<digits>                 e.g. 0, 10, 156
<digits>.<digits>        e.g. 156.0, 156.000, 157.097
.<digits>                e.g. .25 (a quarter of a second)
```

Formally:

- A `time` is:

  - one or more digits, optionally followed by `.` and one or more digits, **or**
  - a leading `.` followed by one or more digits.

Notes:

- There is **no limit** on the number of digits before or after the decimal separator.
- Tools may normalize times (e.g., fixed 3 decimal places) but must preserve semantics.
- When times are used as anchors in utterances, they must appear as **standalone tokens** separated from other tokens by whitespace.

### 6.2 Basic anchor patterns (without `|`)

Anchors appear as numeric tokens **separated by spaces** from IPA fragments. There is no `|` syntax around them.

Common patterns (spaces shown as `·` for clarity):

1. **Bounded fragment (start and end)**

   ```text
   156.000·bõʒuɾ ma bɛlə!·156.800
   ```

   Means: the IPA fragment starts at `156.000` s and ends at `156.800` s.

2. **Start anchor only**

   ```text
   156.000·bõʒuɾ ma bɛlə!
   ```

   Means: the fragment begins at `156.000` s; the end time is not explicitly anchored.

3. **End anchor only**

   ```text
   bõʒuɾ ma bɛlə!·156.800
   ```

   Means: the fragment ends at `156.800` s; the start time is not explicitly anchored.

4. **Inline anchor inside a word**

   ```text
   bø·158.120·nwa
   ```

   Here `158.120` marks a precise boundary **between** the IPA sequences `bø` and `nwa`. The two fragments belong to the same orthographic word but have distinct timing.

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
- The pipe character `|` is used **only** in the pause marker `||` and nowhere else in the syntax.  
  Inside IPA fragments and annotations, `|` is just another character (see §5.3).

### 6.4 General anchor sequences in an utterance

An utterance body is effectively a **sequence of time anchors, pauses, IPA fragments, and annotations**.

Example:

```text
@charlotte: 157.097 bõʒuɾ [en souriant] 157.600 158.088 bø 158.120 nwa[trainant] 159.00
```

Tokenized conceptually as:

- Anchor `157.097`
- Fragment `bõʒuɾ [en souriant]`
- Anchor `157.600`
- Anchor `158.088`
- Fragment `bø`
- Anchor `158.120`
- Fragment `nwa[trainant]`
- Anchor `159.00`

Semantics:

1. **Fragments and their anchors**

   - Each IPA fragment may have:
     - A **start anchor**: the closest anchor **immediately before** it on the same line.
     - An **end anchor**: the closest anchor **immediately after** it on the same line.

   - If both exist, the fragment’s duration is fully defined.
   - If only one exists, only the start or end time is known.
   - If no anchor exists, no timing is defined for that fragment.

2. **Shared anchors**

   - A time anchor can serve as the:
     - End time of the fragment before it, and
     - Start time of the fragment after it.

   In the previous example:

   - `bø` is between 158.088 s and 158.120 s.
   - `nwa[trainant]` is between 158.120 s and 159.00 s.

3. **Pauses**

   A pause is written as:

   ```text
   tStart || tEnd
   ```

   - It is a **special token** that defines a silent interval.
   - It contains no IPA fragment and does not share anchors with adjacent fragments, except that:
     - `tStart` may equal the end anchor of the fragment before the pause.
     - `tEnd` may equal the start anchor of the fragment after the pause.

### 6.5 Anchor monotonicity (per sentence)

For **Strict TIPA**:

- Within a single utterance line, consider all times in order of appearance that are:
  - standalone time anchors, and
  - left/right endpoints of pauses (`t1` and `t2` in `t1 || t2`).

For any consecutive pair of anchors `(t_i, t_{i+1})` used to delimit a fragment or pause, the linter must ensure:

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
  - time anchors (tokens matching the `time` syntax)
  - pauses (`t1 || t2`)
  - IPA fragments
  - Annotations (`[...]`)

The first `# ` or `## ` that satisfies the comment rules in §3.1 starts an inline comment to the end of the line.

### 7.2 Multiple spans per line

An utterance may contain multiple anchored regions and pauses:

```text
@charlotte: 157.097 bõʒuɾ [en souriant] 157.600 158.088 bø 158.120 nwa[trainant] 159.00
```

- Parsers should treat the entire body as a single temporal sequence.
- Whitespace outside IPA/annotations is a separator and may be normalized.

### 7.3 Utterances without anchors

Anchors are optional:

```text
@benoit: bõʒuɾ ma bɛlə!
```

- Such utterances are valid but have **no temporal information**.
- In Strict TIPA, these lines are allowed; the monotonicity constraint is vacuously true (no anchors).

---

## 8. Grammar (informative EBNF)

The following grammar is **informative**, not the only possible implementation, but captures the intended structure.

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
                  [ whitespace, inline-comment ],
                  ;

role-prefix     = role-name, whitespace*, ":", whitespace* ;

role-name       = "@", role-id ;
role-id         = 1*( nonspace-char - ":" - "=" ) ;

utterance-body  = { utterance-token, [ whitespace ] } ;

utterance-token = pause
                | time-anchor
                | fragment ;

pause           = time, whitespace*, "||", whitespace*, time ;

time-anchor     = time ;

fragment        = { ipa-char-or-annotation }+ ;

ipa-char-or-annotation
                = annotation
                | ipa-char ;

annotation      = "[", { annotation-char }, "]" ;

ipa-char        = unicode-char
                  - line-break
                  - "["
                  - "]"
                  - "\\"
                | escape-seq ;

annotation-char = unicode-char
                  - line-break
                  - "["
                  - "]"
                  - "\\"
                | escape-seq ;

escape-seq      = "\\", ( "[" | "]" | "\\" ) ;

time            = digit, { digit }, [ ".", digit, { digit } ]
                | ".", digit, { digit } ;

inline-comment  = "#", [ "#" ], " ",
                  { any-char-except-line-break } ;
```

Notes:

- This grammar treats each numeric `time` as a `time-anchor` token.
- The semantics (mapping anchors and pauses to fragments) follow §6.4–§6.5.
- Implementations may tokenize differently as long as they preserve the same observable behavior.

---

## 9. Strict TIPA profile (“Strict IPA”)

A **Strict TIPA** (a.k.a. *Strict IPA*) document is a normalized form that a linter can produce automatically.

A linter that transforms a TIPA file into Strict TIPA must apply the following rules:

### 9.1 Role injection

For every utterance line that does **not** start with `@roleId:` or `@roleId =`:

1. Prepend the default role identifier `@0: `.
2. The content of the line after injection must be unchanged (except for normalization of surrounding whitespace).

Example input:

```text
156.000 bõʒuɾ ma bɛlə! 156.800
```

Strict TIPA output:

```text
@0: 156.000 bõʒuɾ ma bɛlə! 156.800
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
   - As the left/right endpoints of pauses (`t1 || t2`).

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

### 9.5 Escaping invariants

In Strict TIPA:

- Inside IPA fragments and annotations, the characters `[` and `]` must appear **only** as:
  - `\[` to denote a literal `[`, or
  - `\]` to denote a literal `]`.
- Backslash `\` must appear **only** as:
  - `\\` (literal backslash),
  - `\[` or `\]` (as above).
- No other escaping is required or allowed:
  - Sequences such as `\|`, `\/`, `\:` etc. **must not** appear in Strict TIPA.
  - Characters `/`, `|`, `:`, `@`, `#`, and any standard or extended IPA symbols must appear literally.
- A linter must:
  - Escape any unescaped `[` or `]` that appear inside IPA fragments or annotations.
  - Remove obsolete or unnecessary escapes for other characters.

---

## 10. Examples

### 10.1 Simple dialogue with roles and timing

```text
@benoit = A man
@charlotte = A woman

# Greeting
@benoit: 156.000 bõʒuɾ ma bɛlə! 156.800
@charlotte: 157.097 bõʒuɾ [en souriant] 157.600 158.088 bø 158.120 nwa[trainant] 159.00
```

### 10.2 Example with a pause

```text
@0 = Default role

@0: 10.000 ɔlə 10.300 10.300 || 10.800 10.800 sa va 11.200
```

Semantics:

- `ɔlə` from 10.000 s to 10.300 s
- Silence from 10.300 s to 10.800 s
- `sa va` from 10.800 s to 11.200 s

### 10.3 Escaping inside IPA and annotations

```text
@spk1 = Narrator

@spk1: 0.000 i di \[meta\] pa:se / eksperimɑ̃ 2.500 [note: includes literal \[ and \] and / and |]
```

Interpretation:

- The fragment `i di [meta] pa:se / eksperimɑ̃` runs from 0.000 s to 2.500 s.
- Inside the annotation, `\[` and `\]` give literal `[` and `]`.
- Slash `/` and pipe `|` are used directly without escaping.

---

## 11. Summary

- TIPA is a UTF‑8 text format for IPA transcripts with:
  - Multi‑role attribution using `@roleId` and `@roleId:`
  - Temporal anchors expressed as real values in seconds, written as standalone numeric tokens
  - Pauses with `t1 || t2`
  - Inline annotations in `[ ... ]`
  - Inline and whole-line comments using `# ` and `## `
  - Escaping of **only** `[` `]` and `\` inside IPA fragments and annotations
- The format supports **full IPA and extended IPA** (extIPA, tone letters, prosodic symbols, etc.) as plain Unicode characters, without special escaping.
- The **Strict TIPA** profile defines a canonical form suitable for tooling and storage, enforcing:
  - Explicit roles for every utterance (`@0:` when none is given)
  - Role declarations for all used roles
  - Monotonic anchor times within each utterance
  - Minimal, consistent escaping of `[` `]` and `\`, with `/` and `|` never needing escapes inside IPA or annotations.



© 2025 Benoit Pereira da Silva – Licensed under Creative Commons Attribution 4.0 International (CC BY 4.0).
