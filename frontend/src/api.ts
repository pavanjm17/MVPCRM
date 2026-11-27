const BASE = "/api/v1";

export const STAGES = ["NEW", "CONTACTED", "MEETING", "PROPOSAL", "WON", "LOST"] as const;
export const PRODUCTS = ["TERM", "ENDOWMENT", "ULIP", "MONEY_BACK", "PENSION", "HEALTH"] as const;
export const FREQS = ["MONTHLY", "QUARTERLY", "HALF_YEARLY", "YEARLY"] as const;

export interface Lead { id: number; name: string; phone: string; email: string; city: string; state: string;
  product: string; stage: string; premiumBudget: number; notes: string; createdAt: string }
export interface Policy { id: number; leadId: number; leadName: string; insurer: string; policyNumber: string;
  product: string; sumAssured: number; annualPremium: number; frequency: string; nextDueDate: string }
export interface Followup { id: number; leadId: number; leadName: string; dueDate: string; note: string; done: boolean }
export interface Dashboard { stageCounts: Record<string, number>; renewalsDue30d: number; renewalPremiumDue30d: number;
  annualPremiumBook: number; followupsDueOrOverdue: number }

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, { headers: { "Content-Type": "application/json" }, ...init });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error ?? `Request failed (${res.status})`);
  }
  return res.status === 204 ? (undefined as T) : res.json();
}
const send = (method: string, body?: unknown): RequestInit => ({ method, body: body ? JSON.stringify(body) : undefined });

export const api = {
  dashboard: () => req<Dashboard>("/dashboard"),
  leads: (q = "", stage = "") => req<Lead[]>(`/leads?q=${encodeURIComponent(q)}&stage=${stage}`),
  saveLead: (l: Partial<Lead>) => req<Lead>(l.id ? `/leads/${l.id}` : "/leads", send(l.id ? "PUT" : "POST", l)),
  deleteLead: (id: number) => req<void>(`/leads/${id}`, send("DELETE")),
  policies: () => req<Policy[]>("/policies"),
  addPolicy: (p: Partial<Policy>) => req<Policy>("/policies", send("POST", p)),
  deletePolicy: (id: number) => req<void>(`/policies/${id}`, send("DELETE")),
  followups: () => req<Followup[]>("/followups"),
  addFollowup: (f: Partial<Followup>) => req<Followup>("/followups", send("POST", f)),
  doneFollowup: (id: number) => req<void>(`/followups/${id}/done`, send("PATCH")),
};

export const inr = (n: number) => new Intl.NumberFormat("en-IN", { style: "currency", currency: "INR", maximumFractionDigits: 0 }).format(n);
export const label = (s: string) => s.replace("_", " ").toLowerCase().replace(/^\w/, (c) => c.toUpperCase());
