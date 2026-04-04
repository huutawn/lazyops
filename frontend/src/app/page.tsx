const dayThreeChecklist = [
  'Standalone Next.js 16 app scaffold',
  'TypeScript strict mode and path aliases',
  'Baseline lint, typecheck, test, and build scripts',
  'Environment example and local development README',
];

export default function HomePage() {
  return (
    <main className="page-shell">
      <section className="hero-card">
        <p className="eyebrow">LazyOps Frontend</p>
        <h1>Day 3 scaffold is in place.</h1>
        <p className="lede">
          This app is the starting point for the operator console that will
          support onboarding, targets, integrations, deployments, topology, and
          observability.
        </p>

        <div className="meta-grid">
          <article className="meta-card">
            <span className="meta-label">Framework</span>
            <strong>Next.js 16</strong>
          </article>
          <article className="meta-card">
            <span className="meta-label">Language</span>
            <strong>TypeScript strict</strong>
          </article>
          <article className="meta-card">
            <span className="meta-label">Package manager</span>
            <strong>npm</strong>
          </article>
          <article className="meta-card">
            <span className="meta-label">Next step</span>
            <strong>Day 4 architecture</strong>
          </article>
        </div>

        <div className="status-panel">
          <div className="status-chip">Status: Bootstrap ready</div>
          <ul className="checklist">
            {dayThreeChecklist.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </div>
      </section>
    </main>
  );
}
