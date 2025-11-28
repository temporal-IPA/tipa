# PTIPA (Pré‑Temporal IPA) File Format Specification

- **Version**: 1.0 (draft)
- **Author**: Benoit Pereira da Silva
- **Date**: 28/11/2025

---

## 1. Purpose and scope

PTIPA (Pré‑Temporal IPA, “Pre‑Temporal IPA”) is a plain‑text format that combines:

- **UTF‑8 textual fragments** (orthographic or other plain text)
- **Temporal anchors** (timestamps in seconds)
- **Multiple speaker / role attribution**
- **Inline annotations** (e.g. stage directions or prosodic notes)
- **Inline comments**

It is intended as a **pre‑phonetic companion** to TIPA:

- PTIPA carries **what is said** in ordinary text.
- TIPA carries **how it sounds** in IPA/extIPA.

The primary goal of PTIPA is to enable tools to:

1. Store and edit timestamped textual transcriptions in a simple text file.
2. Feed those transcriptions into phonetizers that produce TIPA documents.
3. Keep the structural layer (roles, timing, annotations, comments) identical between pre‑ and post‑phonetic representations.

PTIPA is designed to be:

- Easy to read and write by humans
- Straightforward to parse by tools
- **Structurally aligned** with TIPA so that both formats can co‑exist in the same toolchain.

---

## 1.1 Relationship to TIPA

PTIPA reuses the **entire structural layer** of TIPA v1.0:

- Role declarations and utterance prefixes (`@roleId`, `@roleId:`)
- Temporal anchors expressed as real values in seconds (`<digits>.<digits>`)
- Optional `|` delimiters around anchored fragments
- Pauses expressed as `tStart || tEnd`
- Inline annotations in `[ ... ]`
- Whole‑line and inline comments using `# ` and `## `
- The **Strict profile** with role injection and monotonic anchor checks

The only conceptual difference is the **nature of the fragments** between anchors:

- In TIPA, fragments are **IPA/extIPA phonetic strings**.
- In PTIPA, fragments are **plain text strings** (typically orthographic).

### 1.1.1 Compatibility guarantee

PTIPA is intentionally defined so that:

> **Every syntactically valid TIPA v1.0 document is also a valid PTIPA v1.0 document.**

Concretely:

- PTIPA parsers must treat any fragment that looks like IPA/extIPA simply as **text**.
- Tools are free (but not required) to **special‑case IPA‑looking fragments** (e.g. to skip phonetization) without changing the PTIPA syntax.

This means a single file can be:

- Interpreted as **TIPA** by phonetic tools.
- Interpreted as **PTIPA** by pre‑/post‑processing tools that only care about structure and text.

---

## 1.2 Non‑intrusive semantics

PTIPA inherits TIPA’s core design principle:

> **Inside fragments, the format must be invisible.**

Concretely:

- Inside text fragments, PTIPA does **not** introduce any escape character.
- Characters such as `/`, `\`, `|`, `@`, `:`, `#` are always taken **literally** inside fragments.
- Structural syntax (roles, anchors, pauses, annotations, comments) lives **around** fragments, not inside them.

The only reserved characters inside fragments are:

- `[` and `]`, which are reserved for annotations and therefore **cannot** appear inside a fragment.
- Line breaks, which terminate a fragment.

As in TIPA, if literal brackets are needed inside content, they should be written using alternative glyphs (`⟦ ⟧`, spelled out words, etc.) or carried via annotations rather than inside fragments.

---

## 2. File format basics

Unless explicitly overridden in this document, **all rules from TIPA v1.0 apply unchanged**.

### 2.1 Encoding

- A PTIPA file **must** be encoded in UTF‑8.
- Text fragments may contain **any Unicode characters**, including:
  - Latin, Cyrillic, Greek, Asian scripts, etc.
  - Digits, punctuation and symbols.
  - IPA and extIPA characters (they simply count as text).

PTIPA does not constrain the language or script; phonetizers are expected to use external metadata (annotations, file name, project settings, etc.) to decide **how** to interpret the text.

### 2.2 Lines and line breaks

Exactly as in TIPA:

- The document is a sequence of **lines** separated by `LF` (`
`), `CRLF` (`
`) or `CR` (``).
- Leading and trailing whitespace on a line is ignored **except** inside:
  - text fragments
  - annotations (`[...]`)

### 2.3 Line types

Every non‑empty, non‑whitespace line is exactly one of:

1. **Comment line**
2. **Role declaration line**
3. **Utterance line**

The syntax of each of these is identical to TIPA v1.0 and summarized again below.

---

## 3. Comments

Comments behave exactly as in TIPA:

- A **whole‑line comment** is a line whose first non‑whitespace characters are:

  ```text
  # <space>...
  ## <space>...
  ```

- An **inline comment** can appear after an utterance on the same line:

  ```text
  @benoit: 12.000 | Bonjour ma belle ! | 12.600  # inline comment
  @charlotte: 13.097 | Bonjour [en souriant] | 13.600  ## stronger comment
  ```

Parsing rules (identical to TIPA):

- The first `#` that:
  - is **not** inside an annotation (`[...]`), and
  - is either the first non‑whitespace character on the line, **or** is preceded by whitespace,
  - and is followed by a space (`# ` or `## `)

  starts a comment that runs to the end of the line.

Everything from that `#` (or `##`) to the end of the line is ignored by parsers.

As in TIPA, the pipe `|` has no special interaction with comments.

---

## 4. Roles

Roles in PTIPA are identical to roles in TIPA.

### 4.1 Role names and declarations

- A role name is introduced with `@` and must not contain spaces.
- A role declaration line has the form:

  ```text
  @<roleId> = <optional description...>
  ```

  Example:

  ```text
  @benoit = A man
  @charlotte = A woman
  ```

### 4.2 Utterance role prefix

A PTIPA utterance may be prefixed by a role:

```text
@benoit: 12.000 | Bonjour ma belle ! | 12.600
@charlotte: 13.097 | Bonjour [en souriant] | 13.600
```

- The first colon `:` after the role name is treated as the separator.
- Whitespace around the colon is allowed.

### 4.3 Utterances without explicit role

As in TIPA, utterance lines may omit any role prefix:

```text
12.000 | Bonjour ma belle ! | 12.600
```

In the **Strict PTIPA** profile, these lines are treated as belonging to a synthetic role `@0` (see §8.1).

---

## 5. Text fragments, annotations and characters

### 5.1 Text fragments

A **text fragment** is the content between anchors and annotations, e.g.:

```text
Bonjour ma belle !
Je suis ravi de te revoir.
OK, ça marche.
i did meta passe / experimantation
```

Fragments may contain:

- Plain orthographic words
- Spaces and punctuation (`! ? , . ; : …` etc.)
- Digits and symbols
- Arbitrary Unicode characters, including IPA/extIPA graphemes

PTIPA does **not** distinguish between tokens and sub‑tokens:

- `Bonjour ma belle !` may be treated as a single fragment, or
- `Bonjour`, `ma`, `belle !` may be separate fragments

depending only on how the author places anchors and annotations.

As in TIPA:

- **No escaping is ever required or recognized inside fragments.**
- Characters like `/`, `|`, `\`, `@`, `#`, `:` are all taken literally.
- The only characters that **cannot** appear inside a fragment are:
  - `[` and `]` (reserved for annotations)
  - Line breaks

### 5.2 Mixed text and IPA

PTIPA allows **mixed content** inside fragments:

```text
Je dis "bonjour" [fr-FR] → / bɔ̃ʒuʁ /
```

Here:

- `Je dis "bonjour" [fr-FR] → / bɔ̃ʒuʁ /` is a single text fragment.
- The `/ bɔ̃ʒuʁ /` sequence is syntactically just text.

Tools are free to apply heuristics, such as:

- “Fragments that look like `/.../` in IPA are already phonetic; do not re‑phonetize.”
- “Fragments that contain only IPA/extIPA from the TIPA grapheme inventory are IPA; others are orthographic.”

However, such heuristics are **out of scope** of the PTIPA syntax and must not affect parsing.

### 5.3 Annotations

Annotations are identical to those in TIPA and use square brackets:

```text
[en souriant]
[voice=whisper]
[lang=fr-FR]
[prosody: fast, rising F0]
```

Rules:

- Syntax: `[annotation text]`
- They may appear:
  - Between words
  - Between fragments
  - Adjacent to anchors and `|` delimiters

Example:

```text
@charlotte: 13.097 | Bonjour [en souriant] | 13.600
```

Inside annotations:

- All characters are taken literally, except:
  - `]`, which closes the annotation
  - Line breaks, which are not allowed

Annotations are the recommended place for:

- Language codes: `[lang=fr-FR]`
- Orthographic hints: `[orth=fr-1990]`
- Phonetizer options: `[phon=fr-casual]`

These values are **conventions**, not part of the core syntax.

### 5.4 No escape sequences

PTIPA inherits TIPA’s “no escape” rule:

- Backslash `\` is never an escape.
- Sequences like `\[`, `\]`, `\`, `\|`, `\/` have **no special meaning**.
- If a literal `[` or `]` is required in a fragment, it must be encoded indirectly (e.g. via annotations or alternative glyphs).

This makes PTIPA robust for both text and mixed text/IPA without complex escaping rules.

---

## 6. Temporal anchors, pipes and pauses

Anchors, pipes and pauses behave **exactly** as in TIPA. This section restates the behavior with text examples.

### 6.1 Time format

A **time anchor** is a non‑negative real value in seconds with the textual form:

```text
<digits> "." <digits>
```

Examples:

- `0.250`
- `10.0`
- `156.000`
- `13.097`

Formally:

```ebnf
time = digit, { digit }, ".", digit, { digit } ;
```

Times used as anchors must appear as **standalone tokens** separated by whitespace.

### 6.2 Anchors and `|` delimiters

PTIPA accepts both **bare** and **pipe‑guarded** anchor styles.

#### 6.2.1 Bare anchor syntax

```text
12.000 Bonjour ma belle ! 12.600
```

Semantics:

- The fragment `Bonjour ma belle !` starts at 12.000 s and ends at 12.600 s.

Variants:

1. **Start and end anchors**

   ```text
   12.000 Bonjour ma belle ! 12.600
   ```

2. **Start anchor only**

   ```text
   12.000 Bonjour ma belle !
   ```

3. **End anchor only**

   ```text
   Bonjour ma belle ! 12.600
   ```

4. **Inline anchor inside a sentence**

   ```text
   12.000 Bonjour ma belle ! 12.600 Ça va ? 13.000
   ```

#### 6.2.2 Pipe‑guarded syntax (recommended)

For readability, anchors may be visually separated using `|`:

```text
12.000 | Bonjour ma belle ! | 12.600
```

Pattern:

```text
<time> | <text + annotations> | <time>
```

Semantics:

- Times are anchors.
- `|` tokens are **pure delimiters** and are not part of the text fragment.
- Their presence or absence does not affect timing.

Rules:

- A **pipe delimiter** is the standalone token `|` (separated by whitespace).
- In **Strict PTIPA**, delimiters are written as `space, '|', space` (` | `).

All other uses of `|` (not as standalone tokens) are treated as ordinary characters inside fragments or annotations.

### 6.3 Pauses

A **pause** is a silent interval between two anchors written as:

```text
tStart || tEnd
```

Example:

```text
10.300 || 10.800
```

Semantics:

- Silence from 10.300 s to 10.800 s.

`||` is a dedicated pause token and is not confused with two `|` delimiters.

### 6.4 General anchor sequences

Within an utterance line, the body is a sequence of:

- Time anchors (`<digits>.<digits>`)
- Pauses (`t1 || t2`)
- Optional pipe delimiters (`|`)
- Text fragments
- Annotations (`[...]`)

For each fragment:

- **Start time**: closest time anchor immediately before the fragment on the same line.
- **End time**: closest time anchor immediately after the fragment on the same line.

As in TIPA, anchors may be shared between neighboring fragments.

### 6.5 Anchor monotonicity (per sentence)

In the **Strict PTIPA** profile (§8), anchors and pause endpoints in one utterance must be **strictly increasing** whenever they delimit a fragment or pause:

```text
t_{i+1} > t_i
```

This is identical to TIPA’s requirement.

---

## 7. Utterance line structure

An utterance line has the structure:

```text
[whitespace][@roleId: ]utterance-body[whitespace][# or ## inline comment]
```

The utterance body is the sequence described in §6.4.

Multiple anchored regions, pauses and annotations may appear on a single line; whitespace outside fragments and annotations may be normalized by tools.

This structure is identical to TIPA, with “IPA fragments” replaced by “text fragments”.

---

## 8. Strict PTIPA profile

The **Strict PTIPA** profile provides a canonical representation, mirroring the Strict TIPA profile.

### 8.1 Role injection

For every utterance line that does **not** start with `@roleId:` or a role declaration:

1. Prepend `@0: `.
2. Optionally ensure that there is a declaration for `@0`:

   ```text
   @0 = Default role
   ```

### 8.2 Role declarations

For each role used as `@roleId:` in at least one utterance:

- Ensure at least one declaration line of the form:

  ```text
  @roleId = <description>
  ```

If missing, a Strict PTIPA linter may insert empty descriptions.

### 8.3 Anchor consistency

Within each utterance:

1. Collect all times appearing as anchors or pause endpoints.
2. In textual order, for every pair `(t_i, t_{i+1})` delimiting a fragment or pause enforce:

   ```text
   t_{i+1} > t_i
   ```

Violations must be reported as errors or warnings; Strict PTIPA output should not be emitted without correcting times.

### 8.4 Structural invariants

In Strict PTIPA:

- Fragments must **not** contain `[` or `]`.
- There are **no escape sequences**; backslash `\` is never special.
- Pipes used as anchor delimiters must appear in canonical form:

  ```text
  12.000 | Bonjour ma belle ! | 12.600
  ```

- Whitespace:
  - Outside fragments and annotations, tools may normalize to single spaces.
  - Inside fragments and annotations, whitespace must be preserved verbatim.

All of these rules are inherited directly from Strict TIPA and keep the formats aligned.

---

## 9. Interoperability with TIPA and conversion workflows

### 9.1 Using one file as PTIPA and TIPA

Because TIPA and PTIPA share the same structural layer:

- The **same file** can be:
  - treated as **PTIPA** when running text‑centric tools (segmentation, normalization, metadata extraction),
  - treated as **TIPA** when running phonetic tools.

This is possible because PTIPA’s parser:

- Does not attempt to interpret fragments beyond “arbitrary UTF‑8 text minus `[` and `]`”.
- Accepts all IPA/extIPA characters without special casing.

### 9.2 Typical PTIPA ⇒ TIPA pipeline

A typical workflow is:

1. **Record & align**

   - Produce a PTIPA file where fragments are orthographic transcriptions aligned to audio with anchors and pauses.

2. **Phonetize**

   - For each PTIPA fragment, a phonetizer:
     - Determines language and style (via annotations, role metadata, project settings, etc.).
     - Converts the text to IPA/extIPA.
     - Optionally injects fine‑grained anchor points inside fragment boundaries.

3. **Emit TIPA**

   - Write a TIPA file with identical roles, anchors, pauses, annotations and comments, but:
     - Replace each PTIPA fragment with one or more IPA/extIPA fragments.
     - Optionally refine the temporal structure (e.g. add more anchors inside words).

Because the syntax is shared, tools can keep references between PTIPA and TIPA at the level of:

- Line number
- Role
- Anchor pairs

without inventing extra identifiers.

### 9.3 Round‑tripping

Nothing in PTIPA forbids **round‑tripping**:

- A TIPA file can be treated as PTIPA.
- A text‑normalization tool may rewrite fragments (e.g. case‑folding, spell‑checking) while keeping the PTIPA structure.
- A phonetizer can then generate a new TIPA file.

The only caution is to preserve anchors and roles to maintain alignment.

---

## 10. Examples

### 10.1 Simple dialogue (PTIPA only)

```text
@benoit = A man
@charlotte = A woman

# Greeting
@benoit: 12.000 | Bonjour ma belle ! | 12.600
@charlotte: 13.097 | Bonjour [en souriant] | 13.600 13.800 | Ben oui, ça va. | 14.400
```

### 10.2 Mixed PTIPA/TIPA fragments in one file

```text
@spk1 = Narrator

# Text fragment, to be phonetized later
@spk1: 0.000 | Je dis "meta passe expérimentation". | 3.000

# IPA fragment passed through as-is (already phonetic)
@spk1: 3.000 | / meta pas ekspeʁimɑ̃tasjɔ̃ / | 5.000
```

Both lines are valid PTIPA. The second line is also a perfectly valid TIPA utterance.

### 10.3 PTIPA with a pause

```text
@0 = Default role

@0: 10.000 | Allô | 10.300 10.300 || 10.800 10.800 | ça va ? | 11.200
```

Semantics:

- `Allô` from 10.000 s to 10.300 s
- Silence from 10.300 s to 10.800 s
- `ça va ?` from 10.800 s to 11.200 s

---

## 11. Summary

PTIPA is a **pre‑phonetic sibling** of TIPA:

- It keeps the **same structural model**:
  - Roles
  - Anchors and pauses
  - Annotations and comments
  - Strict profile and monotonicity checks
- It only changes the **interpretation of fragments**:
  - from “IPA/extIPA phonetic strings” (TIPA)
  - to “arbitrary plain‑text strings” (PTIPA)

By guaranteeing that **every TIPA file is also a valid PTIPA file**, PTIPA allows:

- Clean separation between textual transcription and phonetic realization.
- Simple pipelines that move from raw text to IPA and back.
- Shared tooling for editing, linting and aligning transcripts, independently of whether they are already phonetized.

© 2025 Benoit Pereira da Silva – Licensed under Creative Commons Attribution 4.0 International (CC BY 4.0).
