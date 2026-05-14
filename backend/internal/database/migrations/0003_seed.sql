-- Idempotent seed. Re-running is safe: ON CONFLICT (slug) DO NOTHING on every insert.

-- Roles --------------------------------------------------------------------
INSERT INTO categories (slug, name, kind, description, icon, sort_order) VALUES
    ('frontend',        'Frontend Developer',  'role',  'JS, React, browser APIs, performance.',           'monitor',     10),
    ('backend',         'Backend Developer',   'role',  'APIs, databases, concurrency, reliability.',      'server',      20),
    ('fullstack',       'Full-Stack Developer','role',  'Frontend + backend + everything between.',        'layers',      30),
    ('system-architect','System Architect',    'role',  'Scalable distributed systems and trade-offs.',    'network',     40),
    ('devops',          'DevOps / SRE',        'role',  'Pipelines, infra, reliability, observability.',   'workflow',    50)
ON CONFLICT (slug) DO NOTHING;

-- Topics -------------------------------------------------------------------
INSERT INTO categories (slug, name, kind, description, icon, sort_order) VALUES
    ('javascript',     'JavaScript',          'topic', 'Language fundamentals, async, closures.',       'braces',      10),
    ('react',          'React',               'topic', 'Hooks, rendering, state, performance.',         'atom',        20),
    ('typescript',     'TypeScript',          'topic', 'Generics, narrowing, advanced types.',          'file-code',   25),
    ('css',            'CSS',                 'topic', 'Layout, flex/grid, specificity.',               'palette',     30),
    ('node',           'Node.js',             'topic', 'Event loop, streams, runtime.',                 'hexagon',     35),
    ('go',             'Go',                  'topic', 'Goroutines, channels, idiomatic Go.',           'cog',         40),
    ('databases',      'Databases',           'topic', 'SQL, indexes, transactions, NoSQL.',            'database',    50),
    ('system-design',  'System Design',       'topic', 'Scaling, caching, sharding, queues.',           'network',     60),
    ('docker',         'Docker',              'topic', 'Images, containers, multi-stage builds.',       'box',         70),
    ('kubernetes',     'Kubernetes',          'topic', 'Pods, deployments, services, ingress.',         'ship',        80),
    ('ci-cd',          'CI / CD',             'topic', 'Pipelines, testing, release strategies.',       'git-merge',   90),
    ('security',       'Security',            'topic', 'AuthN/Z, OWASP, secrets management.',           'shield',      100),
    ('behavioral',     'Behavioral',          'topic', 'Conflict, ownership, leadership stories.',      'users',       110)
ON CONFLICT (slug) DO NOTHING;

-- Questions ----------------------------------------------------------------
-- Helper pattern:
--   INSERT INTO questions (slug, title, body, answer, difficulty) ... ON CONFLICT (slug) DO NOTHING;
--   INSERT INTO question_categories ... using SELECTs over slugs (also ON CONFLICT DO NOTHING via PK).

-- JavaScript ---------------------------------------------------------------
INSERT INTO questions (slug, title, body, answer, difficulty) VALUES
('js-closures',
 'Explain closures in JavaScript with a practical example.',
 'Be specific about lexical scope, when closures form, and a use case.',
 'A closure is created when a function references variables from its enclosing lexical scope, and that inner function outlives the outer call. Practical example: a counter factory — the inner increment function closes over a private "count" variable so each returned counter has isolated state. Closures are used for data privacy (module pattern), memoization, currying, and event handlers that need access to setup-time state. Watch out for closures over loop variables with var (use let) and unintended retention of large objects in long-lived closures.',
 'easy'),

('js-event-loop',
 'How does the JavaScript event loop handle async work?',
 'Touch on the call stack, task queue, microtask queue, and rendering.',
 'JS is single-threaded with a call stack. Async APIs (timers, network, fs) are handed off to the host (browser/Node). When the stack empties, the event loop pulls work from queues: first ALL microtasks (Promise.then, queueMicrotask, MutationObserver), then ONE macrotask (setTimeout, setImmediate, I/O). After each macrotask the microtask queue is drained again. In browsers a render step runs between macrotasks if needed. Implication: a Promise chain can starve rendering; setTimeout(0) yields, await of a settled Promise still schedules a microtask. Heavy CPU work needs workers.',
 'medium'),

('js-this-binding',
 'How is the value of "this" determined in JavaScript?',
 'Cover default, implicit, explicit, new, and arrow function rules.',
 'Five rules in priority order: 1) new binding — invoked with new, this is the new object. 2) Explicit — call/apply/bind sets this. 3) Implicit — called as obj.method, this is obj. 4) Default — standalone call, this is undefined in strict mode (else global). Arrow functions don''t have their own this; they capture it lexically from the enclosing scope, so binding rules don''t apply. In classes, methods are not auto-bound — passing them as callbacks loses this unless you bind or use an arrow field.',
 'medium'),

('js-promises-vs-async',
 'When would you prefer async/await over raw Promises?',
 'Discuss readability, error handling, parallelism, and pitfalls.',
 'async/await reads top-down which improves readability and debuggability (stack traces are intact, breakpoints work). Use try/catch for errors instead of .catch chains. Prefer raw Promises (or Promise.all) when you want explicit parallelism — sequential awaits are easy to write but accidentally serialize independent work. Common pitfalls: forgetting await (silent unhandled rejection), awaiting in a forEach (doesn''t wait), and not handling individual failures inside Promise.all (use Promise.allSettled).',
 'medium')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO question_categories (question_id, category_id)
SELECT q.id, c.id FROM questions q, categories c
WHERE (q.slug, c.slug) IN (
    ('js-closures','javascript'),('js-closures','frontend'),('js-closures','fullstack'),
    ('js-event-loop','javascript'),('js-event-loop','frontend'),('js-event-loop','node'),('js-event-loop','backend'),
    ('js-this-binding','javascript'),('js-this-binding','frontend'),
    ('js-promises-vs-async','javascript'),('js-promises-vs-async','node'),('js-promises-vs-async','frontend')
)
ON CONFLICT DO NOTHING;

-- React --------------------------------------------------------------------
INSERT INTO questions (slug, title, body, answer, difficulty) VALUES
('react-reconciliation',
 'How does React decide what to re-render?',
 'Discuss the virtual DOM, keyed diffing, and bail-outs.',
 'React builds a virtual DOM tree on each render of a component. The reconciler compares the new tree against the previous one element-by-element of the same type. Same type → diff props and recurse. Different type → unmount old subtree, mount new. Lists are diffed by key — missing or unstable keys force re-mount. Bail-outs: React.memo, useMemo, and useCallback skip work when inputs are referentially stable. State updates outside the React tree (refs, DOM mutations) don''t trigger renders. Concurrent rendering can interrupt work and resume, but the diff algorithm itself is unchanged.',
 'medium'),

('react-hooks-rules',
 'Why must hooks be called at the top level, in the same order each render?',
 'Explain how React tracks hook state under the hood.',
 'Hooks are tracked by call order, not name — React maintains a per-component linked list of hook slots and advances a pointer on each call. If you call hooks inside a condition or loop, the order changes between renders, the pointer misaligns, and state ends up wired to the wrong hook. Same reason early returns before all hooks are dangerous. The eslint-plugin-react-hooks lint rule catches violations. Custom hooks compose by following the same rules.',
 'medium'),

('react-state-vs-derived',
 'When should something be state vs derived from props/state?',
 'Cover redundancy, single source of truth, and when memoization helps.',
 'If a value can be computed from existing state or props, don''t store it — derive it. Storing duplicates leads to sync bugs and stale data after updates. Compute inline in render; if the computation is expensive, wrap in useMemo. Only promote derived data to state if you need to override it independently (e.g., editable form fields seeded from props). Lift state up when two siblings need the same data; co-locate state when only one component reads it.',
 'easy'),

('react-context-perf',
 'What are the performance pitfalls of React Context?',
 'Discuss re-renders, splitting providers, and alternatives.',
 'Any consumer re-renders when the Provider value changes by reference, even if the part it reads is unchanged. Common pitfalls: passing a fresh object literal each render (wrap in useMemo), bundling unrelated state into one context (split into multiple), or using context for high-frequency updates like cursor position (use a ref, a store like Zustand/Jotai, or useSyncExternalStore for fine-grained subscriptions). React 19''s use(Context) and the upcoming Context selector RFC also help.',
 'hard')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO question_categories (question_id, category_id)
SELECT q.id, c.id FROM questions q, categories c
WHERE (q.slug, c.slug) IN (
    ('react-reconciliation','react'),('react-reconciliation','frontend'),('react-reconciliation','fullstack'),
    ('react-hooks-rules','react'),('react-hooks-rules','frontend'),
    ('react-state-vs-derived','react'),('react-state-vs-derived','frontend'),
    ('react-context-perf','react'),('react-context-perf','frontend')
)
ON CONFLICT DO NOTHING;

-- TypeScript ---------------------------------------------------------------
INSERT INTO questions (slug, title, body, answer, difficulty) VALUES
('ts-unknown-vs-any',
 'What is the difference between unknown and any?',
 'When would you reach for each?',
 'any opts out of the type system — every operation is allowed and errors won''t surface until runtime. unknown is the type-safe counterpart: you must narrow it (typeof, instanceof, a type guard, or a schema parse) before doing anything with it. Use unknown at boundaries where data has no compile-time shape (JSON parsing, third-party callbacks); use any only as a last-resort escape hatch you commit to removing.',
 'easy'),

('ts-generics-constraints',
 'How do you use generic constraints with the extends keyword?',
 'Show a real example and explain the value over plain generics.',
 'extends in a generic position adds a constraint: <T extends { id: string }> only accepts types with an id. This lets the implementation safely access .id while keeping T''s exact shape (e.g., to preserve the caller''s richer type in the return). Combined with keyof you can constrain to property names: function pluck<T, K extends keyof T>(obj: T, key: K): T[K]. The extra precision lets you return narrowed types instead of widening to a generic record.',
 'medium'),

('ts-discriminated-unions',
 'What are discriminated unions and why are they powerful?',
 'Give a use case and contrast with plain unions.',
 'A discriminated (or tagged) union has a common literal property — the discriminant — that lets TS narrow exhaustively. Example: type Result = {kind:''ok'',value:T} | {kind:''err'',error:E}. Switching on kind narrows in each branch. Plain unions force runtime type guards or in-checks. Pair with a never-typed default branch (assertNever) so adding a new variant becomes a compile error at every switch — a refactor-safe pattern.',
 'medium')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO question_categories (question_id, category_id)
SELECT q.id, c.id FROM questions q, categories c
WHERE (q.slug, c.slug) IN (
    ('ts-unknown-vs-any','typescript'),('ts-unknown-vs-any','frontend'),('ts-unknown-vs-any','backend'),
    ('ts-generics-constraints','typescript'),('ts-generics-constraints','frontend'),
    ('ts-discriminated-unions','typescript'),('ts-discriminated-unions','frontend')
)
ON CONFLICT DO NOTHING;

-- Go -----------------------------------------------------------------------
INSERT INTO questions (slug, title, body, answer, difficulty) VALUES
('go-goroutines-vs-threads',
 'How are goroutines different from OS threads?',
 'Touch on scheduling, stack size, and the M:N model.',
 'Goroutines are user-space, multiplexed onto a small pool of OS threads by the Go runtime scheduler (M:N model). They start with a tiny ~2KB stack that grows segmented on demand, vs an OS thread''s fixed multi-MB stack — so you can run millions of goroutines cheaply. The scheduler cooperatively preempts at function calls and channel ops (with non-cooperative preemption since Go 1.14). Trade-off: blocking syscalls park the underlying thread, so heavy syscalls can starve other goroutines unless GOMAXPROCS / additional Ms scale up.',
 'medium'),

('go-channels-vs-mutex',
 'When would you use a channel vs a mutex in Go?',
 'Be explicit about ownership semantics and pitfalls.',
 'Mutex protects shared memory — multiple goroutines mutating the same struct. Channels communicate by moving ownership of data between goroutines (the value is conceptually transferred, not shared). Use mutex for caches, counters, in-place state. Use channels for pipelines, fan-out/in, work queues, signaling done/cancel. Pitfalls: deadlocks from unbuffered channels when no receiver exists, goroutine leaks from blocked sends, complex select+timeout logic. Heuristic: "share memory by communicating" — but a mutex with a small critical section is often simpler than a channel.',
 'hard'),

('go-context-usage',
 'How should context.Context flow through a Go program?',
 'Discuss cancellation, deadlines, and what NOT to put in it.',
 'Pass context as the first argument to any function that might block, do I/O, or call other context-aware functions. Derive child contexts with WithCancel/WithTimeout/WithDeadline at the boundary that owns the lifetime (incoming HTTP request, CLI command). Always defer cancel() to release resources. Never store contexts in structs or pass nil; pass context.TODO() if you genuinely don''t have one. Don''t use context.Value for parameters — it''s for request-scoped metadata like auth, trace IDs. Always check ctx.Done() in loops with select.',
 'medium')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO question_categories (question_id, category_id)
SELECT q.id, c.id FROM questions q, categories c
WHERE (q.slug, c.slug) IN (
    ('go-goroutines-vs-threads','go'),('go-goroutines-vs-threads','backend'),
    ('go-channels-vs-mutex','go'),('go-channels-vs-mutex','backend'),
    ('go-context-usage','go'),('go-context-usage','backend')
)
ON CONFLICT DO NOTHING;

-- Databases ----------------------------------------------------------------
INSERT INTO questions (slug, title, body, answer, difficulty) VALUES
('db-index-tradeoffs',
 'What are the trade-offs of adding a database index?',
 'Cover read speed, write cost, storage, and selectivity.',
 'Indexes speed up reads that filter or sort by the indexed columns by allowing the DB to skip full table scans. Costs: every insert/update/delete must also update each index (write amplification), they consume disk and memory in the buffer cache, and the planner can pick a bad index if statistics are stale. Low-selectivity indexes (few distinct values like a boolean) are usually useless. Composite index column order matters — leftmost prefix rule. Covering indexes (INCLUDE clauses) avoid heap lookups for read-heavy queries.',
 'medium'),

('db-transactions-isolation',
 'What problems do transaction isolation levels solve?',
 'Explain dirty read, non-repeatable read, phantom read, and which levels prevent each.',
 'Concurrent transactions can see inconsistent state. Phenomena: dirty read (see uncommitted writes), non-repeatable read (same SELECT returns different rows mid-tx), phantom read (new rows appear matching a previous predicate), and write skew (consistent reads + concurrent writes violate an invariant). SQL standard levels: READ UNCOMMITTED allows all; READ COMMITTED blocks dirty; REPEATABLE READ blocks dirty + non-repeatable; SERIALIZABLE blocks all. Postgres defaults to READ COMMITTED; its REPEATABLE READ is snapshot isolation (still allows write skew); SERIALIZABLE uses SSI to detect and abort conflicting txns.',
 'hard'),

('db-n-plus-1',
 'What is the N+1 query problem and how do you fix it?',
 'Give a concrete example and the fix.',
 'N+1 happens when you fetch a list of N parents, then issue one more query per parent to fetch its children — 1 + N queries total. Example: SELECT * FROM users; then for each user SELECT * FROM posts WHERE user_id = ?. Fixes: JOIN both in one query, use IN with the collected ids and group in app code, or use eager-loading features in your ORM (preload/with). For graph-shaped data, batching DataLoaders coalesce concurrent calls into single round-trips.',
 'easy')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO question_categories (question_id, category_id)
SELECT q.id, c.id FROM questions q, categories c
WHERE (q.slug, c.slug) IN (
    ('db-index-tradeoffs','databases'),('db-index-tradeoffs','backend'),('db-index-tradeoffs','system-design'),
    ('db-transactions-isolation','databases'),('db-transactions-isolation','backend'),('db-transactions-isolation','system-design'),
    ('db-n-plus-1','databases'),('db-n-plus-1','backend')
)
ON CONFLICT DO NOTHING;

-- System Design ------------------------------------------------------------
INSERT INTO questions (slug, title, body, answer, difficulty) VALUES
('sd-cache-strategies',
 'Compare cache-aside, write-through, and write-behind caching.',
 'Trade-offs and when each fits.',
 'Cache-aside (lazy loading): app checks cache, on miss reads DB and populates cache. Simple, resilient to cache outages, but suffers thundering herd on miss and risk of stale reads after writes. Write-through: every write goes to cache and DB synchronously. Cache is always consistent with DB but writes are slower. Write-behind (write-back): write hits cache, async flush to DB. Fastest writes but data loss risk if cache fails. Pair with TTL and explicit invalidation. Use cache-aside for read-heavy general workloads, write-through where consistency is critical, write-behind for write-heavy buffered systems.',
 'medium'),

('sd-rate-limiting',
 'How would you design a rate limiter for an HTTP API?',
 'Discuss algorithms and distributed considerations.',
 'Algorithms: token bucket (smoothes bursts, refill rate r, capacity b), leaky bucket (fixed output rate), fixed window counter (simple but boundary spikes), sliding window log (accurate, expensive memory), sliding window counter (approximation). For a distributed API, store counters in Redis with atomic ops (INCR + EXPIRE, or Lua scripts for token bucket). Key by user id / api key / IP depending on policy. Return 429 with Retry-After. Watch for clock skew, Redis as SPOF (use replicas), and graceful degradation if Redis is down (fail open vs fail closed depends on threat model).',
 'hard'),

('sd-idempotency',
 'How do you make a payment / write API idempotent?',
 'Discuss idempotency keys and edge cases.',
 'Client generates a unique idempotency key per logical operation and sends it in a header. Server stores (key → result) for some TTL (e.g., 24h). On retry: if key exists with completed result, return cached response; if in-progress, return 409 or wait; if new, process and persist atomically with the result. Persist BEFORE running side effects when possible, or use the same DB transaction. Hash request body into the record so reusing a key with a different body returns a conflict instead of an unrelated success. Idempotent operations at the data layer (UPSERT, conditional writes) compose well.',
 'hard')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO question_categories (question_id, category_id)
SELECT q.id, c.id FROM questions q, categories c
WHERE (q.slug, c.slug) IN (
    ('sd-cache-strategies','system-design'),('sd-cache-strategies','backend'),('sd-cache-strategies','system-architect'),
    ('sd-rate-limiting','system-design'),('sd-rate-limiting','backend'),('sd-rate-limiting','system-architect'),
    ('sd-idempotency','system-design'),('sd-idempotency','backend'),('sd-idempotency','system-architect')
)
ON CONFLICT DO NOTHING;

-- Docker -------------------------------------------------------------------
INSERT INTO questions (slug, title, body, answer, difficulty) VALUES
('docker-image-vs-container',
 'What is the difference between a Docker image and a container?',
 'And when do you use multi-stage builds?',
 'An image is an immutable, layered filesystem template plus metadata (entrypoint, env, exposed ports). A container is a running (or stopped) instance of an image with a writable top layer, a process tree, namespaces, and cgroups. Many containers can run from the same image. Multi-stage builds let you compile in a heavy builder stage (full compiler, build tools) and copy only the final artifact into a minimal runtime stage (alpine, distroless). Benefits: smaller final image, fewer vulnerabilities, faster pulls, no build tools shipped to prod.',
 'easy'),

('docker-layer-caching',
 'How does Docker layer caching work and how do you optimize a Dockerfile?',
 'Discuss order of instructions and cache invalidation.',
 'Each instruction in a Dockerfile creates a layer; subsequent builds reuse layers as long as their inputs and parent layers are unchanged. Once a layer is invalidated every layer below it is rebuilt. Optimization: put rarely-changing instructions first (FROM, system deps), copy lock files and run dep install before copying app source, then COPY source code last. Use .dockerignore to exclude node_modules, .git, test data. For monorepos, use BuildKit''s --mount=type=cache and --mount=type=bind to share caches across builds without baking them into layers.',
 'medium'),

('docker-volumes-vs-bind',
 'When do you use a Docker named volume vs a bind mount?',
 'Discuss portability, performance, and ownership.',
 'Bind mount: map a host path directly into the container — great for local dev (hot reload of source code) but tied to host paths and can hit permission issues. Named volume: managed by Docker, stored under /var/lib/docker/volumes, portable across hosts via volume drivers, and the right choice for persistent prod data like a database directory. Bind mounts on macOS/Windows go through a virtualized FS and are slower for large file trees; named volumes are native. For configs, prefer --read-only bind mounts or secrets.',
 'medium')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO question_categories (question_id, category_id)
SELECT q.id, c.id FROM questions q, categories c
WHERE (q.slug, c.slug) IN (
    ('docker-image-vs-container','docker'),('docker-image-vs-container','devops'),('docker-image-vs-container','backend'),
    ('docker-layer-caching','docker'),('docker-layer-caching','devops'),('docker-layer-caching','backend'),
    ('docker-volumes-vs-bind','docker'),('docker-volumes-vs-bind','devops'),('docker-volumes-vs-bind','backend')
)
ON CONFLICT DO NOTHING;

-- Kubernetes ---------------------------------------------------------------
INSERT INTO questions (slug, title, body, answer, difficulty) VALUES
('k8s-pod-vs-deployment',
 'Pod vs Deployment vs Service — what are they and how do they relate?',
 'Be concrete about responsibilities.',
 'Pod: the smallest deployable unit, one-or-more containers sharing network and storage. Pods are mortal; if a node dies, pods die with it. Deployment: a controller that declaratively manages ReplicaSets, which manage Pods. It handles rollouts, rollbacks, and replica counts. Service: a stable virtual IP + DNS name that load-balances to a set of Pods selected by labels. The Service decouples consumers from the churn of Pod lifecycles. You apply a Deployment, it creates Pods, and a Service routes traffic to those Pods.',
 'easy'),

('k8s-readiness-vs-liveness',
 'What is the difference between readiness and liveness probes?',
 'Give an example of when each matters.',
 'Liveness probe: "is this process alive enough to keep running?" If it fails, the container is restarted. Use for deadlocks or unrecoverable state. Readiness probe: "is this instance ready to serve traffic?" If it fails, the Pod is removed from Service endpoints (not restarted). Use during slow startup, while warming caches, or when a dependency is unavailable. A common mistake is using liveness for things readiness should handle — a temporary DB outage causes liveness restarts that don''t fix anything and amplify the problem. Start with readiness only; add liveness conservatively.',
 'medium'),

('k8s-resource-requests-limits',
 'What do CPU/memory requests and limits actually do?',
 'Distinguish scheduling vs runtime enforcement.',
 'Requests: the amount the scheduler reserves on a node — used for placement decisions and as the floor for QoS class. Limits: the upper bound enforced by the kernel at runtime. CPU limit throttles (CFS quotas); memory limit triggers OOMKill when exceeded. Requests = Limits is "Guaranteed" QoS — safest from eviction. Setting only Requests is "Burstable". Setting neither is "BestEffort" — first to be evicted under pressure. Common gotcha: CPU throttling at limits hurts tail latency more than people expect; many teams set requests but leave CPU unlimited.',
 'hard')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO question_categories (question_id, category_id)
SELECT q.id, c.id FROM questions q, categories c
WHERE (q.slug, c.slug) IN (
    ('k8s-pod-vs-deployment','kubernetes'),('k8s-pod-vs-deployment','devops'),('k8s-pod-vs-deployment','system-architect'),
    ('k8s-readiness-vs-liveness','kubernetes'),('k8s-readiness-vs-liveness','devops'),
    ('k8s-resource-requests-limits','kubernetes'),('k8s-resource-requests-limits','devops')
)
ON CONFLICT DO NOTHING;

-- CI/CD --------------------------------------------------------------------
INSERT INTO questions (slug, title, body, answer, difficulty) VALUES
('cicd-blue-green-vs-canary',
 'Compare blue-green, canary, and rolling deployments.',
 'When to choose each, and rollback story.',
 'Rolling: replace instances of the old version with new gradually inside one environment. Cheap, no extra infra; rollback re-rolls the old version. Risk: mixed-version traffic during rollout. Blue-green: stand up a full second environment (green) with the new version; cut traffic over atomically; old (blue) stays as instant rollback target. Doubles infra cost briefly; clean cut-over. Canary: route a small % of real traffic to the new version, observe metrics, then ramp. Great for risk reduction with proper SLO/error monitoring; needs traffic splitting (load balancer, service mesh, feature flags) and automated rollback triggers.',
 'medium'),

('cicd-secrets-in-pipelines',
 'How do you manage secrets in CI/CD pipelines safely?',
 'Discuss storage, rotation, and short-lived credentials.',
 'Never commit secrets — use the CI provider''s encrypted secret store (GitHub Actions Secrets, Vault) and mount at runtime as env vars or files. Mask them in logs. For cloud access prefer OIDC federation: the CI runner exchanges its OIDC token for short-lived cloud credentials, so no long-lived keys exist. Rotate regularly and scope to the least privilege needed for the job. Separate production from preview environments — preview pipelines must not have prod secrets. Audit which workflows can access which secrets; treat pull_request workflows from forks as untrusted.',
 'hard')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO question_categories (question_id, category_id)
SELECT q.id, c.id FROM questions q, categories c
WHERE (q.slug, c.slug) IN (
    ('cicd-blue-green-vs-canary','ci-cd'),('cicd-blue-green-vs-canary','devops'),('cicd-blue-green-vs-canary','system-architect'),
    ('cicd-secrets-in-pipelines','ci-cd'),('cicd-secrets-in-pipelines','devops'),('cicd-secrets-in-pipelines','security')
)
ON CONFLICT DO NOTHING;

-- Security -----------------------------------------------------------------
INSERT INTO questions (slug, title, body, answer, difficulty) VALUES
('sec-jwt-vs-session',
 'JWT vs server session cookies — which would you choose for a new web app?',
 'Trade-offs around revocation, statelessness, and threats.',
 'Server sessions: server stores session state, client holds an opaque cookie ID. Easy revocation (delete the session), can update state freely, but requires a session store (Redis) and sticky-session or shared-store setup. JWT: signed token holding claims; server verifies signature without lookup. Statelessness is convenient for microservices, but revocation is hard (need a denylist or short TTLs + refresh tokens), tokens can''t be updated mid-life, and putting sensitive data in claims leaks via XSS if stored in localStorage. For browser-facing apps default to short-lived JWT access + long-lived refresh, both in httpOnly+Secure+SameSite cookies. Sessions are perfectly fine and often simpler.',
 'medium'),

('sec-csrf-protection',
 'What is CSRF and how do modern apps prevent it?',
 'Cover the threat and the standard defenses.',
 'CSRF tricks an authenticated user''s browser into submitting a request to your site (cookie sent automatically). Defenses: 1) SameSite=Lax/Strict cookies — biggest practical win, blocks most cross-site sends. 2) CSRF tokens — server issues a per-session token, client echoes it in a header; reject mismatches. 3) Custom request headers + CORS — non-simple requests trigger preflight that attackers can''t forge. 4) Re-authentication for sensitive ops. For SPA + httpOnly cookie auth, SameSite=Lax + a CSRF header (double-submit or synchronizer pattern) is the modern baseline.',
 'medium')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO question_categories (question_id, category_id)
SELECT q.id, c.id FROM questions q, categories c
WHERE (q.slug, c.slug) IN (
    ('sec-jwt-vs-session','security'),('sec-jwt-vs-session','backend'),('sec-jwt-vs-session','fullstack'),
    ('sec-csrf-protection','security'),('sec-csrf-protection','frontend'),('sec-csrf-protection','backend')
)
ON CONFLICT DO NOTHING;

-- Behavioral ---------------------------------------------------------------
INSERT INTO questions (slug, title, body, answer, difficulty) VALUES
('beh-conflict',
 'Tell me about a time you disagreed with a teammate. How did you resolve it?',
 'Use STAR. Show empathy + concrete resolution.',
 'Use STAR (Situation, Task, Action, Result). Pick a real disagreement with technical or product stakes — not a personality clash. Situation: short context. Task: what was at stake and your role. Action: what you specifically did — listen first, restate their position, share data, propose a small experiment to break the tie, or escalate appropriately. Result: outcome with a metric or follow-up. Show empathy and self-awareness ("I learned X, would do Y next time"). Avoid making the teammate the villain or making yourself sound like you "won".',
 'easy'),

('beh-failure',
 'Tell me about a project that failed. What did you learn?',
 'Be honest. Show ownership and growth.',
 'Pick a real failure with real consequences (lost users, missed deadline, regression in prod). Own your part — "I underestimated" not "the team didn''t". Walk through what you''d do differently with specifics (smaller PRs, earlier user testing, better monitoring before launch). End with a concrete change you made afterward that prevented recurrence. Interviewers want to see that you can self-correct without blaming others or sugar-coating the failure into a non-failure.',
 'easy'),

('beh-ownership',
 'Tell me about a time you took ownership of something that wasn''t yours.',
 'Show initiative tied to business outcome.',
 'Pick an incident or gap you noticed (flaky CI, slow on-call rotation, undocumented onboarding, unclaimed broken service). Don''t need permission narrative — show you assessed impact, decided to invest your time, and looped in the right people. End with a measurable outcome: hours saved per week, incident MTTR cut by N%, X new hires onboarded faster. Distinguish from heroics — bonus points if you also made it sustainable (docs, runbook, handover).',
 'medium')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO question_categories (question_id, category_id)
SELECT q.id, c.id FROM questions q, categories c
WHERE (q.slug, c.slug) IN (
    ('beh-conflict','behavioral'),('beh-conflict','frontend'),('beh-conflict','backend'),('beh-conflict','fullstack'),('beh-conflict','system-architect'),('beh-conflict','devops'),
    ('beh-failure','behavioral'),('beh-failure','frontend'),('beh-failure','backend'),('beh-failure','fullstack'),
    ('beh-ownership','behavioral'),('beh-ownership','frontend'),('beh-ownership','backend'),('beh-ownership','fullstack'),('beh-ownership','system-architect')
)
ON CONFLICT DO NOTHING;
