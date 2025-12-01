# TIPA

![](assets/logo.png)

This repository contains: 
1. [TIPA v1.0 specifications](specifications/TIPAv1.0.md) for the "Temporal IPA" format.
2. [PTIPA v1.0 specifications](specifications/PTIPAv1.0.md) for the "Pré Temporal IPA" format.
3. `tipa` a golang [cli and library](pkg/) providing TIPA tools.

# TIPA (Temporal IPA) is a plain‑text format that combines:
- **International Phonetic Alphabet (IPA)** transcriptions
- **Temporal anchors** (timestamps in seconds)
- **Multiple speaker / role attribution**
- **Inline annotations** (e.g. stage directions or prosodic notes)
- **Inline comments**

# PTIPA  (Pré‑Temporal IPA)
is intended as a **pre‑phonetic companion** to TIPA:
- PTIPA carries **what is said** in ordinary text.
- TIPA carries **how it sounds** in IPA/extIPA.

## Licences
- The TIPA & PTIPA [specifications](specifications) files are [licensed under CC BY 4.0.](LICENCE_SPECIFICATIONS.md)
- The `tipa` go module is [licensed under the Apache License 2.0.](LICENCE_CODE.md)![]()