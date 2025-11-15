# Skill: Технологический журнал 1С (1C Tech Log Configuration)

**Version:** 1.0
**Enforcement Level:** HIGH
**Compliance:** CRITICAL

## Overview

This skill guides AI agents in configuring and working with 1C:Enterprise technological journal (технологический журнал). The techlog is a powerful diagnostic tool that records detailed information about platform events, but improper configuration can cause severe performance degradation or disk overflow.

**When to use this skill:**
- Configuring logcfg.xml for error diagnostics
- Setting up performance monitoring
- Analyzing locks and deadlocks
- Troubleshooting 1C platform issues
- Responding to user requests about techlog

---

## CRITICAL COMPLIANCE RULES ⚠️

These rules are MANDATORY and MUST be enforced. Violations can cause production system failures.

### 🔴 MOST CRITICAL RULE - ALWAYS CLEANUP!

**⚠️ AFTER COMPLETING WORK WITH TECHLOG - YOU MUST DISABLE IT OR SWITCH TO MINIMAL MODE ⚠️**

This is **ABSOLUTELY MANDATORY** and **NON-NEGOTIABLE**:

1. **When investigation is complete** - IMMEDIATELY disable techlog:
   - Delete `logcfg.xml` file, OR
   - Rename to `logcfg.xml.disabled`, OR
   - Use MCP tool `disable_techlog`

2. **If continuous monitoring is required** - switch to minimal exception-only mode:
   ```xml
   <?xml version="1.0" encoding="UTF-8"?>
   <config xmlns="http://v8.1c.ru/v8/tech-log">
       <dump create="false"/>
       <log location="D:\1CLogs\Errors" history="48" rotation="period" compress="zip">
           <event>
               <eq property="name" value="excp"/>
           </event>
           <property name="all"/>
       </log>
   </config>
   ```

**Why this is critical:**
- Techlog consumes disk space continuously
- Performance overhead accumulates over time
- Can cause disk overflow if left enabled
- Users often forget about enabled logging
- Production systems MUST NOT run with comprehensive logging indefinitely

**When to remind user:**
- After providing any techlog configuration
- After enabling techlog via MCP tools
- When user says "problem solved" or "investigation complete"
- If techlog has been enabled for >24 hours

**Template reminder message:**
```
⚠️ IMPORTANT: Don't forget to disable techlog when investigation is complete!

To disable:
1. Delete D:\path\to\logcfg.xml, OR
2. Rename it to logcfg.xml.disabled, OR
3. Use MCP tool: disable_techlog

Leaving techlog enabled can cause disk overflow and performance degradation.
```

---

## 🔴 CRITICAL WORKFLOW - HOW TO WORK WITH TECHLOG

**⚠️ THIS IS THE CORRECT PROCESS - FOLLOW THIS SEQUENCE STRICTLY! ⚠️**

The techlog workflow is NOT "enable and leave running". It's a **cycle**: enable → wait/action → **disable** → wait for parsing → query logs → analyze → **verify cleanup**.

### Standard Workflow (Passive Monitoring)

Use when investigating sporadic issues that may occur naturally:

```
1. SAVE original configuration (if exists)
   └─> Read current logcfg.xml file (if exists)
   └─> Store content for restoration later
   └─> If file doesn't exist: Remember "no config" state

2. ENABLE techlog
   └─> Call MCP tool: configure_techlog
   └─> Or: Create logcfg.xml file

3. WAIT for target event to occur
   └─> Monitor user actions
   └─> Wait for error to reproduce
   └─> Duration: minutes to hours (NOT days!)

4. DISABLE techlog IMMEDIATELY
   └─> Call MCP tool: disable_techlog
   └─> This writes disabled config to logcfg.xml
   └─> Critical: DO NOT skip this step!

5. WAIT ~10 seconds for parser to process logs
   └─> Parser needs time to read and load data to ClickHouse
   └─> Typical: 5-15 seconds depending on log volume
   └─> Inform user: "Waiting for log parser to process data..."

6. QUERY logs via MCP tools
   └─> Call get_tech_log with time range
   └─> Analyze results

7. ANALYZE and present findings

8. RESTORE original configuration
   └─> If original had config: Restore saved content
   └─> If original had no config: Delete logcfg.xml
   └─> Returns system to pre-investigation state
   └─> User's original settings are preserved!

9. ✅ MANDATORY FINAL CHECK - Verify restoration:
   └─> Confirm: Original config restored successfully
   └─> EXPLICITLY state to user: "Techlog restored to original state"
   └─> Or if was disabled: "Techlog disabled (was not configured before)"
```

### Active Testing Workflow (Unit Tests)

Use when investigating specific reproducible errors:

```
1. SAVE original configuration (if exists)
   └─> Read current logcfg.xml file (if exists)
   └─> Store content for restoration later
   └─> If file doesn't exist: Remember "no config" state

2. PREPARE unit test that triggers the issue
   └─> Create or identify existing test
   └─> Ensure test reproduces the error reliably

3. ENABLE techlog with specific filters
   └─> Call MCP tool: configure_techlog
   └─> Use narrow filters for the specific event type
   └─> Example: EXCP for exceptions, DBMSSQL for SQL errors

4. RUN unit test to trigger the event
   └─> Execute test in 1C
   └─> Wait for test completion
   └─> Duration: seconds to minutes

5. DISABLE techlog IMMEDIATELY after test completes
   └─> Call MCP tool: disable_techlog
   └─> This writes disabled config to logcfg.xml
   └─> Critical: DO NOT leave enabled between test runs!

6. WAIT ~10 seconds for parser to process logs
   └─> Parser processes techlog files
   └─> Loads data into ClickHouse database
   └─> Say: "Waiting 10 seconds for parser..."

7. QUERY logs via MCP tools
   └─> Call get_tech_log with time range covering the test
   └─> Use narrow time window (e.g., last 5 minutes)

8. ANALYZE results and present findings

9. RESTORE original configuration
   └─> If original had config: Restore saved content
   └─> If original had no config: Delete logcfg.xml
   └─> Returns system to pre-investigation state

10. ✅ MANDATORY FINAL CHECK - Verify restoration:
    ├─> Confirm original config restored successfully
    ├─> If need more tests: Keep original saved, repeat from step 3!
    ├─> When all testing done: Original config must be restored
    └─> Tell user: "Techlog restored to original state"
```

### 🔴 MANDATORY POST-ANALYSIS CHECKLIST

**After completing ANY techlog analysis, you MUST perform this check:**

```
□ Analysis complete and results presented to user?
  └─> If NO: Continue analysis
  └─> If YES: Proceed to next check ↓

□ Was original configuration saved at the start?
  ├─> Check saved config variable/content
  └─> If NOT saved: CRITICAL ERROR! Cannot restore!

□ Is original configuration RESTORED?
  ├─> Check 1: If original had config → Restored saved content?
  ├─> Check 2: If original had NO config → Deleted logcfg.xml?
  └─> If NOT restored: CRITICAL ERROR! Restore NOW!

□ User explicitly informed about restoration?
  └─> Say: "✅ Techlog restored to original state" OR
  └─> Say: "✅ Techlog disabled (was not configured before)" OR
  └─> Say: "✅ Your original techlog settings have been restored"

□ If investigation continues (need more data):
  └─> Keep original config saved (don't lose it!)
  └─> Re-enable techlog for next iteration
  └─> After final iteration: RESTORE original config
```

**Template final message:**
```
Analysis complete!

Findings:
[Your analysis here]

✅ Techlog status: RESTORED to original state
   Your previous techlog settings have been restored.
   [Or: Techlog is DISABLED (was not configured before)]

   System is back to normal state.
```

---

### 📋 Template: Disabled Techlog Configuration

When using MCP tool `disable_techlog`, it writes this configuration to logcfg.xml:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<config xmlns="http://v8.1c.ru/v8/tech-log">
    <dump create="false"/>
</config>
```

**What this does:**
- Explicitly disables all techlog events
- No events will be logged
- Minimal file size (3 lines)
- Safe to use during 10-second parser wait

**When to use:**
- After enabling techlog for investigation
- Before querying logs (allows parser to finish)
- As temporary state before restoring original config
```

### Example: Investigating Deadlock Error

```
USER: "We're getting deadlock errors in Document.SalesOrder.BeforeWrite()"

AGENT WORKFLOW:

1. Save original config:
   Read logcfg.xml → Store content
   [If file doesn't exist: Remember "no config"]

2. Understand: Need to capture TDEADLOCK events

3. Prepare:
   "I'll configure techlog to capture deadlock events.
    Do you have a unit test that reproduces this?"
   User: "Yes, Test_SalesOrder_ConcurrentWrite"

4. Configure techlog (Template 2: Lock Analysis):
   Call configure_techlog with:
   - cluster_guid, infobase_guid from cluster_map.yaml
   - Config: TLOCK, TTIMEOUT, TDEADLOCK events
   - history="2"

5. Inform:
   "✅ Techlog enabled for deadlock capture.
    Please run Test_SalesOrder_ConcurrentWrite now."

6. Wait for user: "Test completed, got the error"

7. IMMEDIATELY disable:
   Call disable_techlog
   Say: "Disabling techlog..."

8. Wait for parser:
   "Waiting 10 seconds for log parser to process data..."
   [Wait 10 seconds]

9. Query logs:
   Call get_tech_log with:
   - from: 10 minutes ago
   - to: now
   - events: ["TDEADLOCK", "TTIMEOUT", "TLOCK"]

10. Analyze and present:
    "Found deadlock at 14:23:45:
     - Transaction A locked Table _AccumReg123
     - Transaction B locked Table _Document234
     - Both waiting for each other's locks

     Root cause: ..."

11. RESTORE original config:
    Write saved content back to logcfg.xml
    [Or delete if was "no config"]

12. ✅ MANDATORY FINAL CHECK:
    "✅ Analysis complete!
     ✅ Techlog restored to original state.

     You can safely continue. Your previous settings are preserved."
```

### Example: Investigating Slow Performance

```
USER: "System is slow when generating Report.SalesAnalysis"

AGENT WORKFLOW:

1. Save original config:
   Read logcfg.xml → Store content
   [If file doesn't exist: Remember "no config"]

2. No unit test needed (user will run manually)

3. Configure techlog (Template 3: Performance):
   Call configure_techlog with:
   - Config: DBMSSQL + SDBL with duration > 1 second
   - history="2"

4. Inform user:
   "✅ Techlog enabled to capture slow queries.
    Please run the Sales Analysis report now.
    Tell me when it completes."

5. Wait for user: "Report finished, took 45 seconds"

6. IMMEDIATELY disable:
   Call disable_techlog
   Say: "Disabling techlog..."

7. Wait for parser:
   "Waiting 10 seconds for parser to process logs..."
   [Actual wait: 10 seconds]

8. Query logs:
   Call get_tech_log with:
   - from: 5 minutes ago
   - to: now
   - events: ["DBMSSQL", "SDBL"]

9. Analyze:
   "Found 3 slow queries:
    1. SELECT from _AccumReg123 - 12 seconds, no index
    2. JOIN across 5 tables - 8 seconds
    3. Subquery in WHERE - 6 seconds

    Recommendations: ..."

10. RESTORE original config:
    Write saved content back to logcfg.xml
    [Or delete if was "no config"]

11. ✅ MANDATORY FINAL CHECK:
    "✅ Analysis complete!
     ✅ Techlog restored to original state.

     Implement the recommendations above.
     Re-enable techlog after optimizations to verify improvement."
```

### Anti-Pattern: What NOT to Do

❌ **WRONG - Leaving enabled continuously:**
```
1. Enable techlog
2. Tell user: "Techlog is running, it will capture events"
3. [Leave enabled for hours/days]
4. User reports issue later
5. Query logs
6. ❌ Never verified if techlog is still enabled!
```
**Problems:**
- Disk fills up
- Performance degrades
- Logs contain too much noise
- User forgets it's enabled

✅ **CORRECT - Targeted cycles with verification:**
```
1. Enable techlog
2. Capture event (minutes, not hours)
3. DISABLE immediately
4. Wait 10 seconds
5. Query and analyze
6. ✅ VERIFY techlog is disabled
7. Inform user explicitly
```

### Critical Timing Rules

**⚠️ NEVER skip these waits:**

1. **After DISABLE, before QUERY: Wait 10 seconds minimum**
   - Parser reads techlog files on schedule
   - ClickHouse needs time to index data
   - Querying immediately = incomplete/missing data

2. **After ENABLE, before ACTION: Wait 1-2 seconds**
   - Platform needs time to apply logcfg.xml
   - Ensures first events are captured

3. **Between test iterations: DISABLE, wait, re-ENABLE**
   - Don't leave enabled between test runs
   - Prevents log pollution from unrelated events
   - Saves disk space

---

### ❌ NEVER DO THIS:

1. **NEVER enable all events without filters**
   ```xml
   <!-- WRONG - will kill performance -->
   <event>
       <eq property="name" value="dbmssql"/>
   </event>
   ```
   **Why:** DBMSSQL/DBPOSTGRS/DBORACLE/SDBL events generate massive data volumes (gigabytes per hour).

2. **NEVER enable SYSTEM events without tech support instructions**
   ```xml
   <!-- WRONG - for 1C internal use only -->
   <system level="ERROR" />
   ```
   **Why:** Generates excessive log volume and slows the system. Only use if explicitly requested by 1C tech support.

3. **NEVER set history > 48 hours in production**
   ```xml
   <!-- WRONG for production -->
   <log location="C:\logs" history="168">
   ```
   **Why:** Causes disk overflow. Use history="24" or history="48" maximum.

4. **NEVER exceed 20 &lt;log&gt; elements in one config**
   **Why:** Each log element adds overhead. Platform documentation explicitly warns against this.

5. **NEVER place logs on system disk**
   ```xml
   <!-- WRONG - will fill system disk -->
   <log location="C:\Windows\Temp\logs">
   ```
   **Why:** Can cause system failure if disk fills up.

6. **NEVER omit rotation and compress settings**
   ```xml
   <!-- INCOMPLETE - missing rotation -->
   <log location="D:\logs" history="24">
   ```
   **Better:**
   ```xml
   <log location="D:\logs" history="24" rotation="period" compress="zip">
   ```

### ✅ ALWAYS DO THIS:

1. **ALWAYS use duration filters for DB events**
   ```xml
   <!-- CORRECT - only log slow queries -->
   <event>
       <eq property="name" value="dbmssql"/>
       <gt property="duration" value="1000000"/>  <!-- >1 second -->
   </event>
   ```

2. **ALWAYS include EXCP and EXCPCNTX for diagnostics**
   ```xml
   <!-- CRITICAL for error tracking -->
   <event>
       <eq property="name" value="excp"/>
   </event>
   <event>
       <eq property="name" value="excpcntx"/>
   </event>
   ```

3. **ALWAYS specify separate disk for logs**
   - Use D:\, E:\, or other non-system drives
   - Prefer SSD over HDD for performance

4. **ALWAYS configure rotation and compression**
   ```xml
   <log location="D:\1CLogs" history="24" rotation="period" rotationperiod="1" compress="zip">
   ```

---

## Configuration Structure

### Root Element: &lt;config&gt;

```xml
<?xml version="1.0" encoding="UTF-8"?>
<config xmlns="http://v8.1c.ru/v8/tech-log">
    <dump create="false"/>
    <log location="..." history="...">
        <event>...</event>
        <property>...</property>
    </log>
</config>
```

### &lt;log&gt; Element Attributes

| Attribute | Description | Default | Recommendations |
|-----------|-------------|---------|-----------------|
| `location` | Directory path for logs | Required | Use separate disk (D:\, E:\), NOT C:\ |
| `history` | Hours to keep logs | 24 | Max 24-48 for production, 12 for dev |
| `format` | text or json | text | Use json for automated parsing |
| `placement` | folders or plain | folders | folders = hierarchical, plain = flat |
| `rotation` | period or size | period | period = time-based rotation |
| `rotationperiod` | Hours between rotations | 1 | Keep default (1 hour) |
| `rotationsize` | MB size trigger | 100 | For size-based rotation |
| `compress` | none or zip | none | ALWAYS use zip in production |

### &lt;event&gt; Element - Filtering

Events are filtered using logical operators. Multiple `<event>` blocks are combined with OR logic. Conditions within one `<event>` are combined with AND logic.

**Operators:**
- `<eq property="..." value="..."/>` - equals
- `<ne property="..." value="..."/>` - not equals
- `<gt property="..." value="..."/>` - greater than
- `<ge property="..." value="..."/>` - greater or equal
- `<lt property="..." value="..."/>` - less than
- `<le property="..." value="..."/>` - less or equal
- `<like property="..." value="..."/>` - pattern match (slower!)

**Example:**
```xml
<event>
    <eq property="name" value="dbmssql"/>
    <gt property="duration" value="5000000"/>  <!-- >5 seconds -->
</event>
```

### &lt;property&gt; Element - Output Control

Controls which event properties are written to log.

```xml
<!-- Write all properties -->
<property name="all"/>

<!-- Write only SQL text -->
<property name="sql"/>

<!-- Conditional property writing -->
<property name="sql">
    <event>
        <eq property="name" value="dbmssql"/>
        <gt property="duration" value="1000000"/>
    </event>
</property>
```

---

## Event Types (40+ Available)

### 🔴 Critical Events (Always Enable for Error Diagnostics)

| Event | Description | Volume | Filter Needed? |
|-------|-------------|--------|----------------|
| **EXCP** | Exceptions causing crashes | Low | No - always log all |
| **EXCPCNTX** | Context of exceptions | Low | No - always log all |
| **QERR** | Query compilation errors | Low | No - always log all |
| **PROC** | Process lifecycle (start/stop/crash) | Low | No - informational |
| **CONN** | Connection establish/disconnect | Low | No - informational |

### ⚠️ Performance Events (High Volume - MUST Filter!)

| Event | Description | Volume | Required Filter |
|-------|-------------|--------|-----------------|
| **DBMSSQL** | SQL Server queries | VERY HIGH | duration > 1000000 (1 sec) |
| **DBPOSTGRS** | PostgreSQL queries | VERY HIGH | duration > 1000000 |
| **DBORACLE** | Oracle queries | VERY HIGH | duration > 1000000 |
| **DB2** | IBM DB2 queries | VERY HIGH | duration > 1000000 |
| **DBV8DBENG** | File DB queries | HIGH | duration > 500000 (0.5 sec) |
| **SDBL** | SDBL queries (1C query language) | HIGH | duration > 500000 |

### 🔒 Lock Events (For Deadlock Analysis)

| Event | Description | Volume | Filter Needed? |
|-------|-------------|--------|----------------|
| **TLOCK** | Transaction lock management | Medium | Optional: duration filter |
| **TTIMEOUT** | Lock timeout exceeded | Low | No - always important |
| **TDEADLOCK** | Deadlock detected | Low | No - always critical |

### 📊 System Events

| Event | Description | Volume | Filter Needed? |
|-------|-------------|--------|----------------|
| **ADMIN** | Admin actions | Low | No |
| **CLSTR** | Cluster operations | Low | No |
| **SRVC** | Service start/stop | Low | No |
| **SESN** | Session begin/end | Medium | Optional |
| **CALL** | Incoming remote calls | Medium | Optional: duration filter |
| **SCALL** | Outgoing remote calls | Medium | Optional: duration filter |
| **SCOM** | Server context create/delete | Low | No |

### 🧠 Memory Events

| Event | Description | Volume | Filter Needed? |
|-------|-------------|--------|----------------|
| **MEM** | Memory increase events | Medium | Optional: threshold filter |
| **LEAKS** | Memory leaks (config errors) | Low | No - always log |

### 🔍 Special Purpose Events

| Event | Description | Use Case |
|-------|-------------|----------|
| **HASP** | Hardware key access | License troubleshooting |
| **LIC** | License acquire/release | License monitoring |
| **FTEXTUPD** | Full-text search update | FTS performance issues |
| **FTS** | Full-text search v2 events | FTS diagnostics |
| **INPUTBYSTRING** | Input by string mechanism | Search performance |

---

## Common Event Properties

### Universal Properties (all events)

- `name` - Event type (excp, dbmssql, etc.)
- `level` - Severity: TRACE, DEBUG, INFO, WARNING, ERROR
- `duration` - Microseconds (divide by 1000000 for seconds)
- `process` - Process name
- `OSThread` - OS thread ID
- `Usr` - 1C username
- `SessionID` - Session number
- `ConnID` - Connection ID
- `AppID` - Application ID

### Database Event Properties

- `sql` - SQL query text
- `planSQLText` - Query execution plan
- `Rows` - Records returned
- `RowsAffected` - Records modified
- `Database` - Database name
- `Dbms` - DBMS type (DBMSSQL, DBPOSTGRS, etc.)

### Lock Event Properties (TLOCK)

- `Locks` - List of managed locks
- `Regions` - Lock region names
- `WaitConnections` - Connections being blocked

### Exception Properties (EXCP)

- `Exception` - Exception name
- `Descr` - Exception description
- `Context` - Execution context (call stack)

---

## Configuration Templates

### Template 1: Minimal Error Diagnostics

**Purpose:** Track only critical errors
**Volume:** Very low (KB/day)
**Use when:** Production system, minimal overhead required

```xml
<?xml version="1.0" encoding="UTF-8"?>
<config xmlns="http://v8.1c.ru/v8/tech-log">
    <dump create="false"/>
    <log location="D:\1CLogs\Errors" history="48" rotation="period" compress="zip">
        <event>
            <eq property="name" value="excp"/>
        </event>
        <event>
            <eq property="name" value="excpcntx"/>
        </event>
        <event>
            <eq property="name" value="qerr"/>
        </event>
        <property name="all"/>
    </log>
</config>
```

### Template 2: Lock Analysis

**Purpose:** Diagnose deadlocks and lock timeouts
**Volume:** Low-Medium (MB/day)
**Use when:** Users report "Record is locked" errors

```xml
<?xml version="1.0" encoding="UTF-8"?>
<config xmlns="http://v8.1c.ru/v8/tech-log">
    <log location="D:\1CLogs\Locks" history="24" rotation="period" compress="zip">
        <event>
            <eq property="name" value="tlock"/>
        </event>
        <event>
            <eq property="name" value="ttimeout"/>
        </event>
        <event>
            <eq property="name" value="tdeadlock"/>
        </event>
        <event>
            <eq property="name" value="conn"/>
        </event>
        <property name="all"/>
    </log>
    <dbmslocks/>
</config>
```

### Template 3: Performance Analysis

**Purpose:** Find slow SQL queries and SDBL
**Volume:** Medium (100-500 MB/day)
**Use when:** System is slow, need to optimize queries

```xml
<?xml version="1.0" encoding="UTF-8"?>
<config xmlns="http://v8.1c.ru/v8/tech-log">
    <log location="D:\1CLogs\Performance" history="12" rotation="period" rotationperiod="1" compress="zip">
        <!-- Slow SQL queries (>1 second) -->
        <event>
            <eq property="name" value="dbmssql"/>
            <gt property="duration" value="1000000"/>
        </event>
        <!-- Slow SDBL queries (>0.5 second) -->
        <event>
            <eq property="name" value="sdbl"/>
            <gt property="duration" value="500000"/>
        </event>
        <!-- Write SQL text and execution plan -->
        <property name="sql"/>
        <property name="planSQLText"/>
        <property name="Rows"/>
        <property name="duration"/>
    </log>
    <plansql/>
</config>
```

### Template 4: Comprehensive Diagnostics (Short-term only!)

**Purpose:** Full diagnostic for critical production issues
**Volume:** HIGH (1-10 GB/day) ⚠️
**Use when:** Severe production issue, time-limited investigation (max 2-4 hours!)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<config xmlns="http://v8.1c.ru/v8/tech-log">
    <log location="D:\1CLogs\Full" history="4" rotation="period" compress="zip">
        <!-- Exceptions -->
        <event>
            <eq property="name" value="excp"/>
        </event>
        <event>
            <eq property="name" value="excpcntx"/>
        </event>
        <!-- Locks -->
        <event>
            <eq property="name" value="tlock"/>
        </event>
        <event>
            <eq property="name" value="ttimeout"/>
        </event>
        <event>
            <eq property="name" value="tdeadlock"/>
        </event>
        <!-- Slow queries only -->
        <event>
            <eq property="name" value="dbmssql"/>
            <gt property="duration" value="1000000"/>
        </event>
        <event>
            <eq property="name" value="sdbl"/>
            <gt property="duration" value="500000"/>
        </event>
        <!-- Connections and sessions -->
        <event>
            <eq property="name" value="conn"/>
        </event>
        <event>
            <eq property="name" value="sesn"/>
        </event>
        <event>
            <eq property="name" value="proc"/>
        </event>
        <property name="all"/>
    </log>
    <plansql/>
    <dbmslocks/>
</config>
```

**WARNING:** Template 4 generates large volumes. Use for 2-4 hours maximum, then disable or switch to Template 1/2/3!

---

## File Locations

### Windows

**Config file:** `logcfg.xml`
- Server: `C:\Program Files\1cv8\srvinfo\reg_1541\1CV8\conf\logcfg.xml`
- Cluster directory: `<cluster_dir>\conf\logcfg.xml`
- Client (user): `%LOCALAPPDATA%\1C\1cv8\conf\logcfg.xml`

**Default log location (if no config):**
- `%LOCALAPPDATA%\1C\1cv8\logs\`

### Linux

**Config file:**
- Server: `/opt/1cv8/conf/logcfg.xml`
- Client (user): `~/.1cv8/conf/logcfg.xml`

**Default log location:**
- `~/.1cv8/logs/`

---

## File Formats

### Text Format (placement=folders)

```
45:31.831006-1,SCALL,2,level=INFO,process=1cv8,OSThread=13476,...
```

Format: `mm:ss.microsec-duration,EVENT,depth,property=value,...`

### Text Format (placement=plain)

```
2023-08-01T15:01:45.259000-14998,SCALL,0,level=INFO,...
```

Format: `YYYY-MM-DDThh:mm:ss.microsec-duration,EVENT,depth,property=value,...`

### JSON Format

```json
{"ts":"2023-08-02T08:48:05.982000","duration":"46998","name":"SCALL","level":"INFO",...}
```

Each line is a complete JSON object (not an array).

---

## Duration Values

Duration is measured in **microseconds** (μs).

**Conversion table:**
- 1 second = 1,000,000 microseconds
- 0.5 seconds = 500,000 microseconds
- 3 seconds = 3,000,000 microseconds
- 5 seconds = 5,000,000 microseconds

**Filter examples:**
```xml
<!-- Queries longer than 1 second -->
<gt property="duration" value="1000000"/>

<!-- Queries longer than 3 seconds -->
<gt property="duration" value="3000000"/>

<!-- Queries longer than 500ms -->
<gt property="duration" value="500000"/>
```

---

## Pattern Matching with &lt;like&gt;

The `<like>` operator supports wildcard patterns:

**Wildcards:**
- `%` - 0 or more characters
- `_` - exactly 1 character
- `[...]` - one of listed characters
- `[^...]` - NOT one of listed characters
- `\` - escape character

**Examples:**
```xml
<!-- SQL containing "reference" anywhere -->
<like property="sql" value="%reference%"/>

<!-- SQL starting with "SELECT" -->
<like property="sql" value="SELECT%"/>

<!-- SQL ending with specific table -->
<like property="sql" value="%_AccumReg%"/>
```

**Performance Note:** `<like>` is slower than `<eq>`. Use only when necessary.

---

## Agent Instructions

When a user requests techlog configuration, follow this workflow:

### Step 1: Understand Requirements

Ask clarifying questions:
1. What is the goal? (error diagnosis, performance analysis, lock analysis)
2. Is this production or development environment?
3. How long will logging be enabled? (hours, days, permanent)
4. Are there specific events or errors to investigate?

### Step 2: Select Appropriate Template

Based on user responses:
- **Errors only** → Template 1 (Minimal)
- **Lock issues** → Template 2 (Lock Analysis)
- **Performance/slow queries** → Template 3 (Performance)
- **Critical investigation (short-term)** → Template 4 (Comprehensive)

### Step 3: Customize Configuration

1. **Set location** (MUST be separate disk):
   ```xml
   <log location="D:\1CLogs\[Purpose]" ...>
   ```

2. **Set history** based on environment:
   - Production: history="24" or history="48"
   - Development: history="12"
   - Investigation: history="4" (short-term)

3. **Add specific filters** if user mentioned:
   - Specific duration threshold
   - Specific database events
   - Specific users or sessions

4. **ALWAYS add rotation and compress:**
   ```xml
   rotation="period" compress="zip"
   ```

### Step 4: Validate Configuration

Before presenting config, verify:

✅ **MUST checks:**
- [ ] Location is NOT on C:\ drive
- [ ] History ≤ 48 hours (production) or ≤ 12 hours (dev)
- [ ] DB events (DBMSSQL/DBPOSTGRS/SDBL) have duration filters
- [ ] rotation and compress are set
- [ ] Number of &lt;log&gt; elements ≤ 20
- [ ] NO &lt;system&gt; elements (unless explicitly requested by user)

❌ **MUST NOT:**
- DB events without duration filter
- history > 48 in production
- location on C:\ drive
- Missing rotation/compress

### Step 5: Present Configuration

Provide:
1. Complete XML configuration
2. File path where to save it
3. Expected log volume estimate
4. How long to keep it enabled
5. **⚠️ CRITICAL: Reminder to disable when complete** (see template below)

**ALWAYS include this warning after presenting configuration:**
```
⚠️ IMPORTANT: When investigation is complete, you MUST disable techlog!

To disable:
1. Delete the logcfg.xml file, OR
2. Rename it to logcfg.xml.disabled, OR
3. Call MCP tool: disable_techlog

Leaving techlog enabled indefinitely will cause disk overflow and performance issues.
Set a calendar reminder to check if logging is still needed after [X hours/days].
```

### Step 6: MCP Tool Integration

When using MCP tools `configure_techlog` or `disable_techlog`:

**Before calling configure_techlog:**
1. Read `configs/cluster_map.yaml` to get cluster_guid and infobase_guid
2. Prepare configuration following templates above
3. Validate configuration against MUST checks
4. Call tool with validated config

**Example:**
```
1. Read configs/cluster_map.yaml
2. Extract cluster_guid: "af4fcd7c-0a86-11e7-8e5a-00155d000b0b"
3. Extract infobase_guid: "d723aefd-7992-420d-b5f9-a273fd4146be"
4. Prepare config (Template 1, 2, 3, or 4)
5. Call configure_techlog with cluster_guid, infobase_guid, and config
```

**To disable:**
```
Call disable_techlog with cluster_guid and infobase_guid
```

---

## Troubleshooting

### Issue: Logs not appearing

**Causes:**
1. Config file in wrong location
2. Syntax error in XML
3. Events not matching filters
4. Server not restarted (some scenarios)

**Solutions:**
1. Verify file path (see File Locations section)
2. Validate XML syntax (must be well-formed)
3. Start with `<property name="all"/>` to see all events
4. Check default log location (%LOCALAPPDATA%\1C\1cv8\logs\)

### Issue: Disk filling up

**Causes:**
1. No compress setting
2. History too long
3. No rotation
4. DB events without duration filter

**Solutions:**
1. Add `compress="zip"` immediately
2. Reduce history to 12-24 hours
3. Add `rotation="period"` with `rotationperiod="1"`
4. Add duration filters to all DB events (>1000000)

### Issue: System slowdown

**Causes:**
1. Too many events enabled
2. DB events without filters
3. &gt;20 log elements
4. SYSTEM events enabled

**Solutions:**
1. Remove unnecessary events
2. Add duration filters (>1000000 for DB events)
3. Consolidate log elements (max 20)
4. Remove &lt;system&gt; elements unless required by tech support

### Issue: Not capturing needed events

**Causes:**
1. Filters too restrictive
2. Wrong event name
3. Property not enabled

**Solutions:**
1. Relax duration filter temporarily (e.g., >500000 instead of >1000000)
2. Check event name spelling (case-insensitive but must match exactly)
3. Add `<property name="all"/>` to capture all properties

---

## Best Practices Summary

### ✅ DO:
1. **⚠️ DISABLE TECHLOG WHEN INVESTIGATION IS COMPLETE** - Most important rule!
2. **⚠️ After EVERY analysis - verify techlog is disabled or minimal** - MANDATORY checklist!
3. **⚠️ Always remind user to disable after presenting any configuration**
4. **Follow the workflow:** enable → action → disable → wait 10s → query → analyze → verify
5. Start with Template 1 (minimal) and expand if needed
6. Use duration filters for all DB events (DBMSSQL, DBPOSTGRS, SDBL, etc.)
7. Set history to 24-48 hours maximum in production
8. Always enable rotation and compress
9. Place logs on separate SSD drive (D:\, E:\)
10. Monitor disk space when techlog is enabled
11. Include EXCP and EXCPCNTX for error tracking
12. Use JSON format for automated parsing
13. Document why techlog is enabled and when to disable
14. Test configuration in dev environment first

### ❌ DON'T:
1. **⚠️ Leave techlog enabled after investigation is complete** - CRITICAL!
2. **⚠️ Skip post-analysis checklist verification** - MANDATORY after every analysis!
3. **⚠️ Forget to warn user about disabling** - MANDATORY in every response!
4. **⚠️ Query logs immediately after disable** - Wait 10 seconds for parser!
5. Enable all events without filters
6. Enable SYSTEM events without tech support guidance
7. Set history > 48 hours in production
8. Place logs on C:\ drive
9. Forget rotation and compress settings
10. Create > 20 &lt;log&gt; elements
11. Enable DB events without duration filter
12. Leave comprehensive logging (Template 4) enabled for > 4 hours
13. Enable techlog permanently without business justification
14. Ignore disk space warnings

---

## Enforcement Validation

Before any configuration is presented to the user or sent to MCP tools, it MUST pass these validations:

```python
# Pseudo-code validation
def validate_techlog_config(config, response_text):
    errors = []

    # MOST CRITICAL: Check if disable warning is included
    if "disable" not in response_text.lower() or "отключ" not in response_text.lower():
        errors.append("CRITICAL: Response MUST include warning to disable techlog when complete!")

    # CRITICAL validations
    if config.log.location.startswith("C:\\"):
        errors.append("CRITICAL: Log location MUST NOT be on C: drive")

    if config.log.history > 48 and environment == "production":
        errors.append("CRITICAL: History MUST NOT exceed 48 hours in production")

    for event in config.events:
        if event.name in ["dbmssql", "dbpostgrs", "dboracle", "db2", "sdbl"]:
            if not has_duration_filter(event):
                errors.append(f"CRITICAL: Event {event.name} MUST have duration filter")

    if not config.log.rotation:
        errors.append("CRITICAL: rotation MUST be set")

    if not config.log.compress:
        errors.append("WARNING: compress should be set to 'zip'")

    if len(config.log_elements) > 20:
        errors.append("CRITICAL: MUST NOT exceed 20 <log> elements")

    if has_system_events(config):
        errors.append("CRITICAL: SYSTEM events MUST NOT be enabled without tech support approval")

    return errors
```

If ANY CRITICAL error is found, the configuration MUST be rejected and corrected before proceeding.

---

## References

1. Platform Documentation: Section 3.24 "logcfg.xml" (Appendix 3)
2. ITS: Настройка технологического журнала (https://its.1c.ru/db/metod8dev/content/3474/hdoc)
3. ITS: Просмотр технологического журнала (https://its.1c.ru/db/metod8dev/content/5896/hdoc)

---

## Version History

**1.0** (2025-11-15)
- Initial skill creation
- Comprehensive event reference (40+ types)
- 4 configuration templates
- Critical compliance rules
- Enforcement validation logic
- MCP tool integration guidance
