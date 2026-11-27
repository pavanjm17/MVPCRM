import { FormEvent, useCallback, useEffect, useState } from "react";
import { api, Dashboard, Followup, FREQS, inr, label, Lead, Policy, PRODUCTS, STAGES } from "./api";

type Tab = "Dashboard" | "Leads" | "Policies" | "Follow-ups";
const TABS: Tab[] = ["Dashboard", "Leads", "Policies", "Follow-ups"];

function useLoad<T>(fn: () => Promise<T>, deps: unknown[] = []) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState("");
  const reload = useCallback(() => { fn().then(setData).catch((e) => setError(e.message)); }, deps); // eslint-disable-line
  useEffect(reload, [reload]);
  return { data, error, reload, setError };
}

const fd = (e: FormEvent<HTMLFormElement>) => {
  e.preventDefault();
  return Object.fromEntries(new FormData(e.currentTarget)) as Record<string, string>;
};
const Select = ({ name, options }: { name: string; options: readonly string[] }) => (
  <select name={name}>{options.map((o) => <option key={o} value={o}>{label(o)}</option>)}</select>
);

export default function App() {
  const [tab, setTab] = useState<Tab>("Dashboard");
  return (
    <div className="shell">
      <header>
        <h1>Agent CRM</h1>
        <nav>{TABS.map((t) => <button key={t} className={t === tab ? "on" : ""} onClick={() => setTab(t)}>{t}</button>)}</nav>
      </header>
      <main>
        {tab === "Dashboard" && <DashboardView />}
        {tab === "Leads" && <LeadsView />}
        {tab === "Policies" && <PoliciesView />}
        {tab === "Follow-ups" && <FollowupsView />}
      </main>
    </div>
  );
}

function Err({ msg }: { msg: string }) { return msg ? <p className="err" role="alert">{msg}</p> : null; }

function DashboardView() {
  const { data: d, error } = useLoad<Dashboard>(api.dashboard);
  if (error) return <Err msg={error} />;
  if (!d) return <p>Loading…</p>;
  const total = Object.values(d.stageCounts).reduce((a, b) => a + b, 0);
  return (
    <>
      <section className="stats">
        <div><b>{d.renewalsDue30d}</b><span>Renewals due in 30 days ({inr(d.renewalPremiumDue30d)})</span></div>
        <div><b>{d.followupsDueOrOverdue}</b><span>Follow-ups due or overdue</span></div>
        <div><b>{inr(d.annualPremiumBook)}</b><span>Annual premium in force</span></div>
      </section>
      <h2>Pipeline ({total} leads)</h2>
      <div className="pipeline">
        {STAGES.map((s) => (
          <div key={s} className={`stage s-${s}`} style={{ flexGrow: Math.max(d.stageCounts[s] ?? 0, 0.4) }}>
            <b>{d.stageCounts[s] ?? 0}</b><span>{label(s)}</span>
          </div>
        ))}
      </div>
    </>
  );
}

function LeadsView() {
  const [q, setQ] = useState("");
  const [stage, setStage] = useState("");
  const { data, error, reload, setError } = useLoad<Lead[]>(() => api.leads(q, stage), [q, stage]);
  const run = (p: Promise<unknown>) => p.then(() => { setError(""); reload(); }).catch((e) => setError(e.message));
  const add = (e: FormEvent<HTMLFormElement>) => {
    const f = fd(e); const form = e.currentTarget;
    run(api.saveLead({ ...f, premiumBudget: Number(f.premiumBudget || 0) } as Partial<Lead>).then(() => form.reset()));
  };
  return (
    <>
      <form className="row" onSubmit={add}>
        <input name="name" placeholder="Full name" required />
        <input name="phone" placeholder="Mobile (10 digits)" pattern="[6-9][0-9]{9}" required />
        <input name="city" placeholder="City" />
        <input name="state" placeholder="State" />
        <Select name="product" options={PRODUCTS} />
        <input name="premiumBudget" type="number" min="0" placeholder="Annual budget (₹)" />
        <button>Add lead</button>
      </form>
      <div className="row">
        <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search name, phone or city" />
        <select value={stage} onChange={(e) => setStage(e.target.value)}>
          <option value="">All stages</option>{STAGES.map((s) => <option key={s} value={s}>{label(s)}</option>)}
        </select>
      </div>
      <Err msg={error} />
      {data?.length === 0 && <p className="empty">No leads yet. Add your first lead above.</p>}
      <div className="scroll"><table><thead><tr><th>Name</th><th>Phone</th><th>City</th><th>Product</th><th>Budget</th><th>Stage</th><th></th></tr></thead>
        <tbody>{data?.map((l) => (
          <tr key={l.id}>
            <td>{l.name}</td><td><a href={`tel:+91${l.phone}`}>+91 {l.phone}</a></td><td>{l.city}</td>
            <td>{label(l.product)}</td><td>{inr(l.premiumBudget)}</td>
            <td><select value={l.stage} onChange={(e) => run(api.saveLead({ ...l, stage: e.target.value }))}>
              {STAGES.map((s) => <option key={s} value={s}>{label(s)}</option>)}</select></td>
            <td><button className="ghost" onClick={() => confirm(`Delete ${l.name}?`) && run(api.deleteLead(l.id))}>Delete</button></td>
          </tr>))}</tbody></table></div>
    </>
  );
}

function LeadSelect({ leads }: { leads: Lead[] }) {
  return <select name="leadId" required defaultValue="">
    <option value="" disabled>Select lead</option>{leads.map((l) => <option key={l.id} value={l.id}>{l.name}</option>)}</select>;
}

function PoliciesView() {
  const { data, error, reload, setError } = useLoad<Policy[]>(api.policies);
  const leads = useLoad<Lead[]>(() => api.leads()).data ?? [];
  const add = (e: FormEvent<HTMLFormElement>) => {
    const f = fd(e); const form = e.currentTarget;
    api.addPolicy({ ...f, leadId: Number(f.leadId), sumAssured: Number(f.sumAssured || 0), annualPremium: Number(f.annualPremium || 0) } as Partial<Policy>)
      .then(() => { setError(""); form.reset(); reload(); }).catch((x) => setError(x.message));
  };
  return (
    <>
      <form className="row" onSubmit={add}>
        <LeadSelect leads={leads} />
        <input name="insurer" placeholder="Insurer (e.g. LIC)" required />
        <input name="policyNumber" placeholder="Policy number" required />
        <Select name="product" options={PRODUCTS} />
        <input name="sumAssured" type="number" min="0" placeholder="Sum assured (₹)" />
        <input name="annualPremium" type="number" min="0" placeholder="Annual premium (₹)" />
        <Select name="frequency" options={FREQS} />
        <input name="nextDueDate" type="date" required />
        <button>Add policy</button>
      </form>
      <Err msg={error} />
      {data?.length === 0 && <p className="empty">No policies yet. Add one to start tracking renewals.</p>}
      <div className="scroll"><table><thead><tr><th>Client</th><th>Insurer</th><th>Policy no.</th><th>Sum assured</th><th>Premium</th><th>Next due</th><th></th></tr></thead>
        <tbody>{data?.map((p) => (
          <tr key={p.id}><td>{p.leadName}</td><td>{p.insurer}</td><td>{p.policyNumber}</td><td>{inr(p.sumAssured)}</td>
            <td>{inr(p.annualPremium)} / {label(p.frequency)}</td><td>{p.nextDueDate}</td>
            <td><button className="ghost" onClick={() => api.deletePolicy(p.id).then(reload)}>Delete</button></td></tr>))}</tbody></table></div>
    </>
  );
}

function FollowupsView() {
  const { data, error, reload, setError } = useLoad<Followup[]>(api.followups);
  const leads = useLoad<Lead[]>(() => api.leads()).data ?? [];
  const add = (e: FormEvent<HTMLFormElement>) => {
    const f = fd(e); const form = e.currentTarget;
    api.addFollowup({ leadId: Number(f.leadId), dueDate: f.dueDate, note: f.note })
      .then(() => { setError(""); form.reset(); reload(); }).catch((x) => setError(x.message));
  };
  const today = new Date().toISOString().slice(0, 10);
  return (
    <>
      <form className="row" onSubmit={add}>
        <LeadSelect leads={leads} />
        <input name="dueDate" type="date" required />
        <input name="note" placeholder="What to do (call, send illustration…)" />
        <button>Add follow-up</button>
      </form>
      <Err msg={error} />
      {data?.length === 0 && <p className="empty">Nothing scheduled. Add a follow-up after your next call.</p>}
      <ul className="tasks">{data?.map((f) => (
        <li key={f.id} className={f.done ? "done" : f.dueDate < today ? "late" : ""}>
          <span><b>{f.leadName}</b> – {f.note || "Follow up"}</span><time>{f.dueDate}</time>
          {!f.done && <button onClick={() => api.doneFollowup(f.id).then(reload)}>Mark done</button>}
        </li>))}</ul>
    </>
  );
}
