# VeriFHIR Gateway Scope

## Goal
Build a production-ready HL7v2 to FHIR gateway with quality scoring for healthcare interoperability.

## MVP Scope
- HL7v2 ingestion adapter (MLLP stub now, network adapter next)
- HL7v2 parser with segment-level validation
- Mapping engine from HL7v2 events to FHIR R4 resources
- Quality scoring (completeness and semantic confidence)
- Routing to destination systems (stdout stub now)

## Out of Scope (MVP)
- Full terminology service integration
- Multi-tenant auth and billing
- Full UI dashboard

## Success Criteria
- Parse and map at least ADT A01/A03 messages
- Produce deterministic quality score per message
- Provide auditable processing logs for each transformation step
