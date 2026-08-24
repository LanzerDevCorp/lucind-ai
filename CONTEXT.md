# Lucind AI Orchestration

This context defines the language used to coordinate independent work in a shared repository.

## Language

**Change**:
An independently owned unit of work that remains isolated until an explicit integration decision. It can range from small work completed directly by its Orchestrator to large work delegated across multiple Agents; its identity does not depend on its scale or execution method. A Change can represent a feature, fix, chore, or another kind of repository work.
_Avoid_: Feature, branch, task, or lane when referring to the general unit of independent work

**Orchestrator**:
The provider-independent authority responsible for coordinating one Change and deciding when its work is ready for integration. An Orchestrator can delegate work to multiple collaborating agents without transferring ownership of the Change.
_Avoid_: Agent, executor, or worker

**Ownership Lease**:
The renewable, exclusive claim that allows one Orchestrator to control a Change. Expiration stops new actions, requests cancellation of active Lanes, and preserves their work and evidence; another Orchestrator can acquire ownership only through an explicit, recorded recovery.
_Avoid_: Permanent ownership or silent takeover

**Repository Coordinator**:
The neutral authority that maintains the shared map of Changes, Lanes, Dependencies, and Integration Targets and enforces repository-wide safety rules. It does not make product decisions for an Orchestrator.
_Avoid_: Super-orchestrator or Change owner

**Coordination Scope**:
The boundary within which the Repository Coordinator provides shared authority. The initial stable scope is one local repository on one machine; separate clones and remote machines are outside it.
_Avoid_: Distributed or cross-machine coordination

**Stability Campaign**:
The release-validation process that binds one immutable candidate to three consecutive successful Stability Trials and produces the evidence used to decide whether the core is stable.
_Avoid_: Run or a single test execution

**Stability Trial**:
One complete execution of the stable concurrent-Change journey within a Stability Campaign. A failed Trial resets the Campaign's consecutive-success count.
_Avoid_: Stability Campaign or ordinary Run

**Agent**:
A participant that performs delegated work within a Change and can communicate directly with other Agents in that Change. An Agent cannot independently change scope, priorities, integration decisions, or obligations between Changes; cross-Change effects are coordinated by their Orchestrators.
_Avoid_: Orchestrator or Change owner

**Collaboration Channel**:
An optional provider-specific mechanism that lets Agents exchange information directly. Messages are temporary collaboration; decisions, status, and Dependencies become authoritative only when recorded by the Repository Coordinator.
_Avoid_: Source of truth or durable coordination state

**Shared Memory**:
The durable semantic knowledge available to every Orchestrator and Agent within the Coordination Scope. A required record must be committed to Shared Memory before dependent work can proceed.
_Avoid_: Provider-private context or transient conversation history

**Lane**:
One isolated unit of delegated work within a Change. A code-modifying Lane has its own temporary workspace and branch, and its result must be accepted before it becomes part of the Change.
_Avoid_: Agent, Change, or permanent feature branch

**Acceptance**:
The verified inclusion of a Lane's result into its owning Change under the approved Execution Strategy. Acceptance can occur without additional human confirmation.
_Avoid_: Promotion or final delivery

**Execution Route**:
The provider, model, and profile explicitly authorized for a Lane. Changing the route requires approval from the Lane's Orchestrator.
_Avoid_: Silent provider fallback or inferred quota selection

**Execution Strategy**:
The human-approved method an Orchestrator uses to complete a Change, such as direct work, a structured development flow, or multi-Agent delegation. Initial selection and later changes require confirmation; routine steps within the approved strategy do not.
_Avoid_: Execution Route or an unapproved workflow change

**Write Scope**:
The declared area of the repository that a Lane may modify. Lanes with overlapping Write Scopes must have an explicit execution order and cannot run as independent concurrent work.
_Avoid_: An informal file list or an undeclared editing boundary

**Defect Assessment**:
The classification of a discovered defect by its origin and its effect on the current Lane, other active Lanes, and their Dependencies. Only affected Lanes are blocked; unaffected Lanes continue, and a repair with no active dependents can be scheduled as non-blocking work.
_Avoid_: Treating every failure as an immediate blocking fix

**Defect Record**:
The durable account of a discovered problem, its evidence, impact, and disposition. Every assessed defect must be committed to Shared Memory before remediation can begin, even when no external issue is created.
_Avoid_: Requiring an external tracker for defect history

**Remediation Proposal**:
The prepared fix Change and Dependencies produced from a Defect Assessment, with a recommendation about whether an External Work Item is warranted. One human confirmation selects the external tracking decision and activates remediation; approved work can then proceed within its Execution Strategy.
_Avoid_: Automatically launched repair or unapproved issue creation

**External Work Item**:
An optional representation of work that merits independent tracking, such as a new feature or a defect affecting multiple active Changes. It supports collaboration and traceability but is not required and is not the Repository Coordinator's source of truth.
_Avoid_: Required Change identity or authoritative local state

**Dependency**:
A relationship in which one Change cannot safely continue until another Change is integrated. Completing the required Change makes the blocked Change eligible to continue, but its Orchestrator must approve resumption. A problem caused by the blocked Change remains inside it; a pre-existing or shared problem becomes a separate fix Change.
_Avoid_: Informal waiting or an undocumented blocker

**Integration Target**:
The destination declared when a Change is created that receives its accepted work. It can be a shared branch, a product branch, or the destination required by a dependent Change.
_Avoid_: The currently checked-out branch or an inferred destination

**Promotion**:
The human-confirmed integration of a completed Change into its Integration Target.
_Avoid_: Lane Acceptance or automatic final integration

**Isolated Mode**:
A form of work in which each Change has a separate workspace and can safely coexist with other Changes. It is the default mode for every new Change and remains fixed for the lifetime of that Change.
_Avoid_: Parallel mode

**Exclusive Mode**:
A simplified form of work explicitly selected when one Change uses the primary workspace and no other Change can run concurrently. The selection remains fixed for the lifetime of that Change.
_Avoid_: Legacy mode
