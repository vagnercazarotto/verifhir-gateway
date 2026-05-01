# Healthcare Interoperability Standards Reference

This document is the canonical reference for the standards, terminologies, regulations, and tooling that VeriFHIR Gateway depends on. It exists so contributors do not have to re-discover authoritative sources every time a question comes up.

> Always prefer the official specification URL over a third-party tutorial when validating behavior. If a third-party reference and the official spec disagree, the spec wins.

---

## 1. Standards Development Organizations

### HL7 International

- **Who:** Health Level Seven International — non-profit ANSI-accredited standards body, founded **1987**.
- **What they publish:** HL7 v2.x, HL7 v3 / CDA, FHIR, plus Implementation Guides (IGs).
- **Mission:** "Provide a comprehensive framework and related standards for the exchange, integration, sharing and retrieval of electronic health information."
- **Home:** https://www.hl7.org
- **Confluence (working groups, governance):** https://confluence.hl7.org
- **Community chat (Zulip, free):** https://chat.fhir.org
- **Standards index:** https://www.hl7.org/implement/standards/

### Other relevant SDOs

| Organization | Scope | Site |
| --- | --- | --- |
| IHE (Integrating the Healthcare Enterprise) | Profiles that constrain HL7 / FHIR / DICOM for interop scenarios | https://www.ihe.net |
| ISO TC 215 | International health informatics standards | https://www.iso.org/committee/54960.html |
| DICOM | Medical imaging exchange | https://www.dicomstandard.org |
| SNOMED International | SNOMED CT terminology | https://www.snomed.org |
| Regenstrief Institute | LOINC terminology | https://loinc.org |

---

## 2. HL7 v2.x

The most widely deployed clinical messaging standard in the world. Pipe-and-hat (`|^~\&`) delimited segments. Still dominant for ADT, ORM, ORU, MDM, SIU flows in hospital integration engines.

### History
- v2.1: 1990
- v2.2: 1994
- v2.3 / v2.3.1: 1997 — most widely deployed for legacy ADT
- v2.4: 2000
- v2.5 / v2.5.1: 2003 / 2007 — common modern baseline
- v2.6, v2.7, v2.8: incremental
- v2.9: latest published version

### Where to read the spec
- **Official (HL7 members + free download with registration):** https://www.hl7.org/implement/standards/product_brief.cfm?product_id=185
- **Caristix free HL7 v2 reference (community-maintained, unofficial but accurate for lookup):** https://hl7-definition.caristix.com/v2/

### Message anatomy (quick reference)

```
MSH|^~\&|SendingApp|SendingFac|ReceivingApp|ReceivingFac|20260501120000||ADT^A01|MSG00001|P|2.5
EVN|A01|20260501120000
PID|1||PATID1234^^^HOSPITAL^MR||DOE^JOHN^A||19800101|M
PV1|1|I|ICU^101^1|...
```

- **Segment:** line beginning with a 3-letter code (`MSH`, `PID`, `PV1`, …)
- **Field separator:** `|`
- **Component separator:** `^`
- **Repetition separator:** `~`
- **Escape character:** `\`
- **Subcomponent separator:** `&`
- **Segment terminator:** `<CR>` (`\r`, 0x0D). Some senders use CRLF — VeriFHIR's parser handles both; see [internal/parser](../internal/parser).

### Common message types

| Trigger | Meaning |
| --- | --- |
| ADT^A01 | Admit / visit notification |
| ADT^A03 | Discharge |
| ADT^A04 | Register a patient |
| ADT^A08 | Update patient information |
| ORM^O01 | Order message |
| ORU^R01 | Observation result |
| SIU^S12 | New scheduled appointment |
| MDM^T02 | Document notification with content |

### Transport: MLLP (Minimal Lower Layer Protocol)

HL7 v2 over TCP wraps each message in framing bytes:

```
<VT> ...HL7 message... <FS><CR>
VT = 0x0B   FS = 0x1C   CR = 0x0D
```

Spec context: https://www.hl7.org/documentcenter/public/wg/inm/mllp_transport_specification.pdf

---

## 3. HL7 FHIR (Fast Healthcare Interoperability Resources)

Modern web-friendly standard (REST + JSON/XML/Turtle). Resource-based instead of message-based. Designed by Grahame Grieve, first draft 2011.

### Versions

| Version | Status | Released | Notes |
| --- | --- | --- | --- |
| DSTU1 | retired | 2014 | historical |
| DSTU2 | retired | 2015 | |
| STU3 | mature | 2017 | still in some live deployments |
| **R4** | **Normative (most widely adopted)** | 2019 | use this for production unless you have a specific reason not to |
| R4B | bridge release | 2022 | small additions to R4 |
| R5 | Normative (latest) | 2023 | newer projects can target this |
| R6 | ballot / build | rolling | https://build.fhir.org |

### Authoritative sources

- **Spec home:** https://www.hl7.org/fhir/
- **R4 spec (recommended baseline):** https://hl7.org/fhir/R4/
- **R5 spec:** https://hl7.org/fhir/R5/
- **Continuous build (next version):** https://build.fhir.org
- **Registry of published IGs and packages:** https://registry.fhir.org
- **Terminology server (HL7 reference):** https://tx.fhir.org

### Core concepts

- **Resource:** a discrete, identifiable unit of clinical data (e.g. `Patient`, `Encounter`, `Observation`, `Condition`).
- **Bundle:** a collection of resources (transaction, batch, document, message, searchset).
- **Profile (StructureDefinition):** a constrained version of a base resource for a use case.
- **Extension:** the standard mechanism for adding fields not in the base resource.
- **CodeableConcept:** a value plus the terminology system it came from.
- **Reference:** typed pointer between resources (e.g. `Encounter.subject → Patient`).

### Resources we map from HL7v2 in the MVP

| HL7v2 source | FHIR target |
| --- | --- |
| MSH (envelope) | `MessageHeader` (when modeling as message Bundle) |
| PID | `Patient` |
| PV1 | `Encounter` |
| OBX | `Observation` |
| DG1 | `Condition` |
| AL1 | `AllergyIntolerance` |

---

## 4. HL7 v2 → FHIR Mapping (Official Guidance)

HL7 publishes consensus mappings so vendors do not each invent their own:

- **HL7 v2 to FHIR Implementation Guide:** https://hl7.org/fhir/uv/v2mappings/
- **Per-segment mappings (under the spec):** https://hl7.org/fhir/R4/downloads.html (look for the v2 mappings package)

When in doubt about how a v2 field should land in FHIR, check the IG above first. Our internal mapping rules in [internal/mapping](../internal/mapping) should cite the IG section they implement.

---

## 5. Major Implementation Guides (IGs)

IGs constrain FHIR for specific jurisdictions or use cases. The right IG depends on **where** the data is going.

| IG | Region / Use case | URL |
| --- | --- | --- |
| US Core | United States baseline | https://hl7.org/fhir/us/core/ |
| International Patient Summary (IPS) | Cross-border patient summary | https://hl7.org/fhir/uv/ips/ |
| International Patient Access (IPA) | Patient-facing API | https://hl7.org/fhir/uv/ipa/ |
| SMART App Launch | OAuth2 launch for clinical apps | https://hl7.org/fhir/smart-app-launch/ |
| Bulk Data Access | `$export` for population queries | https://hl7.org/fhir/uv/bulkdata/ |
| Da Vinci (US payer) | Payer/provider exchange | https://hl7.org/fhir/us/davinci-pdex/ |
| CARIN Blue Button | US patient claims | https://hl7.org/fhir/us/carin-bb/ |

---

## 6. Terminologies (Code Systems)

FHIR resources lean on standardized vocabularies. The most important ones:

| Code system | Used for | Authority | URL |
| --- | --- | --- | --- |
| **LOINC** | Lab tests, observations, documents | Regenstrief Institute | https://loinc.org |
| **SNOMED CT** | Clinical findings, procedures, body sites | SNOMED International | https://www.snomed.org |
| **ICD-10 / ICD-11** | Diagnoses, mortality coding | WHO | https://icd.who.int |
| **ICD-10-CM / PCS** | US clinical/procedure coding | CMS / NCHS | https://www.cms.gov/medicare/icd-10 |
| **RxNorm** | US drug normalization | NLM | https://www.nlm.nih.gov/research/umls/rxnorm/ |
| **UCUM** | Units of measure | Regenstrief | https://ucum.org |
| **CVX** | US vaccine codes | CDC | https://www2.cdc.gov/vaccines/iis/iisstandards/vaccines.asp?rpt=cvx |
| **TUSS / TISS** | Brazilian outpatient/billing terminology | ANS | https://www.gov.br/ans/pt-br |
| **CID-10 / CID-11** | Brazilian Portuguese ICD | Ministério da Saúde / WHO | https://icd.who.int |

The HL7 terminology hub (preferred entry point for FHIR ValueSets and CodeSystems): https://terminology.hl7.org

---

## 7. Compliance and Privacy Regulations

VeriFHIR Gateway processes Protected Health Information (PHI). Anything we do has to be defensible against the regulator of the jurisdiction where data is processed.

### Brazil (primary)

- **LGPD — Lei Geral de Proteção de Dados (Lei nº 13.709/2018):** https://www.gov.br/anpd/pt-br
  - Health data is "dado pessoal sensível" (Art. 5, II) — explicit consent or specific legal basis required.
  - Data Protection Officer (Encarregado) is mandatory in many cases.
- **CFM resoluções on telemedicine and electronic medical records:** https://portal.cfm.org.br
- **RNDS — Rede Nacional de Dados em Saúde (Ministério da Saúde):** https://rnds.saude.gov.br
  - Brazil's national health data network, **FHIR R4-based**.
  - Conformance profiles published as IGs — registration via gov.br ecosystem.

### United States

- **HIPAA Privacy & Security Rules:** https://www.hhs.gov/hipaa/
- **ONC Cures Act / 21st Century Cures rule** (mandates FHIR R4 + US Core for certified EHRs): https://www.healthit.gov/topic/oncs-cures-act-final-rule
- **HITRUST CSF** (industry control framework, often required by US payers): https://hitrustalliance.net

### European Union

- **GDPR:** https://gdpr.eu
- **EHDS — European Health Data Space:** https://health.ec.europa.eu/ehealth-digital-health-and-care/european-health-data-space_en
- **MDR (Medical Device Regulation 2017/745)** — relevant if the gateway is classified as medical device software: https://eur-lex.europa.eu/eli/reg/2017/745/oj

### General security baselines worth following

- **NIST SP 800-66 Rev. 2** — HIPAA security implementation guide: https://csrc.nist.gov/pubs/sp/800/66/r2/final
- **OWASP ASVS** — application security verification: https://owasp.org/www-project-application-security-verification-standard/
- **OWASP API Security Top 10:** https://owasp.org/API-Security/

---

## 8. Tooling

### Parsing / runtime libraries

| Tool | Language | Use case | URL |
| --- | --- | --- | --- |
| HAPI FHIR | Java | Reference FHIR server + client | https://hapifhir.io |
| HAPI HL7v2 | Java | HL7 v2 parsing reference impl | https://hapifhir.github.io/hapi-hl7v2/ |
| Firely .NET SDK | C# | FHIR for .NET | https://fire.ly/products/firely-net-sdk/ |
| fhir.js / fhirclient | JS | Browser/Node clients | https://github.com/smart-on-fhir/client-js |
| Mirth Connect / NextGen Connect | Java | Open-source integration engine | https://www.nextgen.com/products-and-services/integration-engine |

### Validators

- **FHIR Validator (Java CLI):** https://github.com/hapifhir/org.hl7.fhir.core/releases — the reference validator the spec authors use.
- **Inferno** (ONC certification test kit): https://inferno.healthit.gov
- **Touchstone** (AEGIS, conformance testing): https://touchstone.aegis.net

### Authoring / browsing IGs

- **Simplifier.net** — FHIR profile registry and authoring: https://simplifier.net
- **Forge** — desktop profile editor: https://fire.ly/products/forge/
- **Trifolia-on-FHIR** (open source IG authoring): https://trifolia-fhir.lantanagroup.com

### Synthetic test data

- **Synthea** — synthetic patient generator (FHIR + C-CDA + CSV): https://github.com/synthetichealth/synthea
- VeriFHIR's own HL7 v2 synthetic generator: see [DATASET-GENERATOR.md](DATASET-GENERATOR.md).

---

## 9. Learning Resources

- **HL7 FHIR Foundation tutorials:** https://www.hl7.org/fhir/overview-dev.html
- **DevDays (yearly FHIR conference, recordings on YouTube):** https://www.devdays.com
- **FHIR Bootcamp (community-curated):** https://confluence.hl7.org/display/FHIR
- **"FHIR for Developers" by Firely (free tutorials):** https://docs.fire.ly
- **Books:**
  - *Principles of Health Interoperability* — Tim Benson & Grahame Grieve.
  - *FHIR for Architects* — series on Firely site.

---

## 10. Versioning and Conformance Policy for VeriFHIR Gateway

To keep the gateway predictable, we commit to the following baseline (revisit per release):

- **HL7 v2 input:** support v2.3.1 through v2.7 envelopes; reject unknown major versions with a clear error.
- **FHIR output:** **R4** as the default target; R5 behind a config flag once we have at least one downstream consumer requesting it.
- **Mappings:** every mapping rule under [internal/mapping](../internal/mapping) must cite the corresponding section of https://hl7.org/fhir/uv/v2mappings/ in a comment or doc.
- **Terminology bindings:** prefer LOINC, SNOMED CT, UCUM, CID-10 (BR) — never invent local code systems without a documented reason.
- **Profiles:** if deployed to RNDS, validate output against the relevant RNDS IG before routing.

Changes to this baseline must update this document in the same PR.

---

## 11. Glossary

| Term | Meaning |
| --- | --- |
| ADT | Admit / Discharge / Transfer (HL7 v2 message family) |
| C-CDA | Consolidated Clinical Document Architecture (HL7 v3 family) |
| CDA | Clinical Document Architecture |
| EHR / EMR | Electronic Health Record / Medical Record |
| FHIR | Fast Healthcare Interoperability Resources |
| HIE | Health Information Exchange |
| IG | Implementation Guide |
| MLLP | Minimal Lower Layer Protocol (HL7 v2 transport) |
| PHI / PII | Protected Health Information / Personally Identifiable Information |
| RNDS | Rede Nacional de Dados em Saúde (Brazil) |
| SDO | Standards Development Organization |
| ValueSet | A FHIR-defined set of permissible codes drawn from one or more code systems |
