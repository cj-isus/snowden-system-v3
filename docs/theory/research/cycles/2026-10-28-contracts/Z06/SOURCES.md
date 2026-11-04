# R28-Z06 — источники и scope прочтения

28.10.2026. Материалы: [REPORT](REPORT.md), [HANDOFF](HANDOFF.md). Public-source captures: `.artifacts/research/r28-contracts/Z06/`.

## 1. Baseline, availability, independence

SECTIONS_READ: R05-009 REPORT §10:545–779 и SOURCES §8; FULL_TEXT_READ
`architecture/RELEASE-EVIDENCE-BOUNDARIES.md`. Root AGENTS/docs AGENTS needed
sections/COMMON/Z06 TASK прочитаны. Scoped AGENTS в ancestors Z06 не найдены.
MAP — существующий pointer, реальное чтение по actual source paths.

Source availability **SOURCE_AVAILABLE**: canon docs и source captures
verifier/Gson доступны, свежие источники получены по exact commit URLs. В
Android .kt/.kts targeted search нет consumer описанного Google status verifier.
Это не отсутствие возможной интеграции во всех dependency/artifact/runtime.

История этого чата содержит авторство baseline D. Новая byte/source проверка
и opposing model не называется внешней независимой рецензией; independent
решение руководителя/C05 отдельно. Одна модель/согласие авторов не primary evidence.

Статусы ниже FULL_TEXT_READ (короткие files), SECTIONS_READ (конкретные части),
FETCH_FAILED различаются. Publication date UNKNOWN, если источник не даёт её;
Last updated не дата создания. Всё read28.10.2026, без downloaded-code execution.

## 2. Source registry

### V — Google/Android Key Attestation Verifier

Commit **34652d3df68b8d6887a6670b00eb0856090dae6d**,
[exact tree](https://github.com/android/keyattestation/tree/34652d3df68b8d6887a6670b00eb0856090dae6d).
Fresh raw captures≈08:56Z; read08:57–09:10Z. Publication date UNKNOWN;
не claimed frozen release/Maven-byte equivalence/installed dependency.

| File | Прочитанная область / следствие |
|---|---|
| src/main/kotlin/GoogleRevocationList.kt | FULL_TEXT_READ1–88: HTTP200/throw/finally, String DTO→REVOKED set, no status age/own timeout |
| src/main/kotlin/Verifier.kt | SECTIONS_READ140–371, result44–68 inspected: three providers; status callback before validate catch; generic Success not operation |
| provider/RevocationChecker.kt | FULL_TEXT_READ1–56: supplied set reference membership; no own immutable copy/status time |
| provider/KeyAttestationCertPath.kt | SECTIONS_READ50–84: dropLast anchor, serial hex formatting |
| provider/KeyAttestationCertPathValidator.kt | SECTIONS_READ118–268; extra-extension guard addressed against source capture: signature/checker sequence; no whole engine/PKI acceptance |
| build.gradle.kts | Targeted manifest dependency/toolchain31–52: Gson2.11.0, Kotlin2.2, JDK21 |

10/10 critical fresh files (5V+5G) совпали с earlier captured bytes; recorded
`source-identity-comparison.json`. New remote byte equality не correctness,
whole-tree atomicity, release attest или product-native PASS.

### G — Google Gson2.11.0

Tag gson-parent-2.11.0: object **3b7c2ad8ce8dd471449ccbaee6d22f64c9816a38**,
peeled commit **828a97be0f8d58108b140b77df8dc76b657f4a87**; tag re-resolved09:05Z.
[Exact tree](https://github.com/google/gson/tree/828a97be0f8d58108b140b77df8dc76b657f4a87).
Fetched≈08:56/09:00Z, SECTIONS_READ08:57–09:09Z, publication date UNKNOWN.

| ID/file под gson/src/main/java/com/google/gson/ | Sections и поддерживаемый факт |
|---|---|
| G1 Gson.java |155–161 defaults;1259–1265 Reader;1345–1381 parse;1466–1475 consumption: nullable/lenient/IO errors and Reader guard |
| G2 internal/bind/ReflectiveTypeAdapterFactory.java |148–161,265–279,498–565: DTO fields повторно assignment, unknown skip/null references; no schema required check |
| G3 internal/bind/TypeAdapters.java + stream/JsonReader.java | STRING458–478,nextString890–921: boolean/numeric coercion; syntax not typed enum |
| G4 internal/bind/MapTypeAdapterFactory.java |172–209: array-pairs and duplicate previous-nonnull guard; no all-object duplicate guarantee |
| G5 internal/ConstructorConstructor.java + GsonBuilder.java |126–176 constructor fallback;builder552–568 strictness: conditional reflection/Unsafe; ready strict parser, not finished TypeAdapter |

[LICENSE](https://github.com/google/gson/blob/828a97be0f8d58108b140b77df8dc76b657f4a87/LICENSE)
FULL_TEXT_READ Apache-2.0. Java/reflection/parser/R8 executions NOT_RUN.
Source null conclusions conditional on ordinary DTO adapter/allocation success,
не promise exact compiled exception/behaviour каждого JVM.

### H — normative HTTP/JSON originals

| ID/author/title/date/URL | SECTIONS_READ28.10, approx08:58–09:13Z | Предел |
|---|---|---|
| H1 R.Fielding/M.Nottingham/J.Reschke eds, IETF RFC9111 HTTP Caching, June2022, [text](https://www.rfc-editor.org/rfc/rfc9111.txt) |§3.3/3.4 incomplete;§4/4.1 URI/method/Vary;§4.2.1/3 estimated age;§4.3.4 304;§5.2.2.2/4;§6 app cache | HTTP freshness not app permission/global latest. Actual headers UNKNOWN |
| H2 T.Bray ed, IETF RFC8259 JSON, Dec2017,[text](https://www.rfc-editor.org/rfc/rfc8259.txt) |§4 duplicate names;§8.1 UTF8;§9 parser limits reference | SHOULD unique не автоматический MUST всех parsers; app schema separate |
| H3 R.Fielding/M.Nottingham/J.Reschke eds, IETF RFC9110 HTTP Semantics, June2022,[text](https://www.rfc-editor.org/rfc/rfc9110.txt) |§8.8.3/1/2 opaque ETag validators, fetched09:09Z | ETag not monotonic status-floor/order |

English originals прочитаны как primary; русский перевод не guessed/claimed.
В REPORT paraphrase/reasoning, captures в ignored artifacts; не full quoted RFC.

### D — official Russian Android status schema

Google/Android Developers, Verify hardware-backed key pairs with key attestation,
[RU](https://developer.android.com/privacy-and-security/security-key-attestation?hl=ru).
Fresh09:00Z capture, actual Russian article SECTIONS_READ≈09:01–09:08Z:
Certificate status schema/Cache-Control/expires/factory-validity/publication.
Last updated **2026-07-09UTC**, original publication UNKNOWN.

Supports REVOKED/SUSPENDED, required status, canonical lowercase serial pattern,
closed properties, expires certificate date, delayed publication. Public root
PEM examples removed from capture; not parsed/verified. Real
`/attestation/status` or `/root` **NOT_FETCHED/LINK_ONLY**. No real certificates,
account/device/latest-status checks; live HTTP headers UNKNOWN.

### J — FasterXML Jackson-core2.18.3

Tag object **d04c20d7ed00f37127e690288f9923e97f6176c7**,
peeled commit **cc34e8d5b75d4c911c16f2bc87d6e13e42eafddd**,
[exact tree](https://github.com/FasterXML/jackson-core/tree/cc34e8d5b75d4c911c16f2bc87d6e13e42eafddd).
Fetched09:00–09:08Z; publication UNKNOWN; no installed runtime/artifact.

| File | SECTIONS_READ / consequence |
|---|---|
| StreamReadFeature.java / JsonParser.java | feature definitions,315–375: explicit duplicate flag false by default; privacy comments version difference checked |
| json/DupDetector.java / JsonReadContext.java | FULL_TEXT_READ DupDetector1–111;context242–253: name check before map/tree, HashSet overhead |
| StreamReadConstraints.java |39–95 defaults;140–190 builder;483–571 checks: document/token unlimited default; named limits/read units |
| JsonFactory.java / TSFBuilder.java |parser refs;builder194–227,350–362: configure duplicate/read constraints |
| UTF8StreamJsonParser.java / ReaderBasedJsonParser.java / ParserBase.java |UTF8_loadMore206–242;Reader248,symbol call map;Base271/1361 duplicate/depth: guards reached; buffering/global RAM not certified |

[LICENSE](https://github.com/FasterXML/jackson-core/blob/cc34e8d5b75d4c911c16f2bc87d6e13e42eafddd/LICENSE)
FULL_TEXT_READ Apache-2.0. Source comment20–30% duplicate overhead — author's
example estimate, not own benchmark and not applied numerically to Snowden.
No parser test or JSON fixture execution.

### N — NetworkNT json-schema-validator1.5.9

Tag1.5.9 → commit **270df62471e6a0c564dc13ae6cf2bf4116916f82**;
[exact tree](https://github.com/networknt/json-schema-validator/tree/270df62471e6a0c564dc13ae6cf2bf4116916f82).
Fetched08:57–08:58Z, read09:00–09:13Z; publication date UNKNOWN.
POM Java8/Jackson2.18.3/itu1.14/slf4j2.0.17; no dependency install/build.

| File | Прочитанное |
|---|---|
| README.md |SECTIONS235–298/config tables,typeLoose/schema retrieval; reference usage, not acceptance |
| TypeValidator.java / utils/JsonNodeUtil.java |FULL_TEXT type1–70 / utils68–120: actual node type check, typeLoose qualifier |
| RequiredValidator.java / EnumValidator.java |FULL_TEXT required1–97 / enum1–124: missing/null different; exact enum unless explicit looseness |
| AdditionalPropertiesValidator.java |35–183: startsWith("#") skip. Narrow candidate limitation, not executed exploit |
| PropertyNamesValidator.java |FULL_TEXT1–68: traverses all field names; useful counterguard for serial-map names, not root/entry unknown fields |
| JsonSchema.java / JsonSchemaFactory.java |599–662 aggregate result;getInstance/constructor/loading branches inspected: error set not operation permit |
| resource DisallowSchemaLoader, DefaultSchemaLoader, SchemaLoaders |FULL_TEXT: local whitelist then terminating deny; ordinary loader lookup fallback potentially network |
| pom.xml / LICENSE |Manifest70–163; LICENSE FULL_TEXT Apache-2.0; entire transitive licence/security audit NOT_DONE |

Archive-source3/3 comparison AdditionalProperties/Disallow/POM equal. Json-Schema
canonical draft07 `.txt` and alternate `.html` source returned403:
**FETCH_FAILED**, attempts stopped; no retry loops/rate bypass/disabled TLS.
No claim «draft spec full read»: official Google published schema + concrete
validator source provide used scope. Generic schema assurance deferred.

### A — atomic/reference/caller primitives

| ID/organisation/URL | SECTIONS_READ / boundary |
|---|---|
| A1 Oracle JDK21 [AtomicReference](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/util/concurrent/atomic/AtomicReference.html) |CAS502–529/updateAndGet638–666: atomic reference identity, retriable side-effect-free callbacks; not file durability or network action. Fetch09:01Z |
| A2 JetBrains [Kotlin collections](https://kotlinlang.org/docs/collections-overview.html) |Collection types/read-only/mutable/val: read-only view not immutable contents. Fetch09:03Z,publication UNKNOWN |
| A3 Oracle JDK21 [URLConnection](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/net/URLConnection.html) / [HttpURLConnection](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/net/HttpURLConnection.html) |timeouts0 infinite,implementation qualification,redirect defaults. Fetch09:09Z; actual custom provider UNKNOWN |
| A4 Oracle JDK21 [CertPathValidator](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/security/cert/CertPathValidator.html) |getInstance algorithm/provider preference vs explicit-provider overload. Fetch09:13Z; actual provider selection NOT_RUN |

Project EncryptedProfileStore.kt **SECTIONS_READ63–144**: expected revision,
generation/floor/uncertainty/pending-stage; copied09:09Z. This is existing
source-only design pattern, not proved disk/current revocation gate. Real
key/profile data untouched. ProfileStoreCrypto captured navigation only,
not full source audit in this task.

## 3. Paired scope и финальный review

C05 TASK read-only. C05 REPORT §§2–3 draft read09:13Z only: same-request inputs,
root/provider/current-generation qualifiers. Own proofs reference primary source,
not agreement. Finished HANDOFF отсутствовал при this review; no wait/poll.
Any subsequent draft mutation not automatically covered by this source cut.

T1–T8 counterexamples: **logical sequence**, not fixtures/field/native proof.
Resource formulas: bounded authored allocation model, not heap/RSS guarantee.
Source acquisitions use verified HTTPS; scripts only transfer/hash/extract public
texts, not execute downloaded programs. No certificate/Google status endpoint,
secret/vault/runtime user log/deploy/test/crypto inputs.

Journals acquisition/reuse-acquisition/additional/parser-path/http-acquisition,
source-identity-comparison,baseline-inputs plus final seal record read dates,
bytes, paths, errors. SHA256 names captured inputs, not correctness/atomic
repository closure. Only Z06/{REPORT,SOURCES,STATUS,HANDOFF}.md and own artifacts
written; old D/chapters/indices/product code/plans/models/roles unchanged.
