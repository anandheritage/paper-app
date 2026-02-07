import { useNavigate } from 'react-router-dom';
import { BookOpen, Search, Library, Bookmark, GraduationCap, FileText, Zap, Globe, Quote, Award } from 'lucide-react';

const FEATURES = [
  {
    icon: Search,
    title: 'Powerful Search',
    description: 'Search millions of papers by title, author, abstract, or topic with rich metadata and citation counts.',
  },
  {
    icon: Quote,
    title: 'Citation Insights',
    description: 'See citation counts and influential citations. Sort by most cited to find landmark papers.',
  },
  {
    icon: Library,
    title: 'Personal Library',
    description: 'Save papers, track your reading progress, and organize your research with tags and notes.',
  },
  {
    icon: Bookmark,
    title: 'Smart Bookmarks',
    description: 'Bookmark papers for quick access. Never lose track of that important reference again.',
  },
  {
    icon: FileText,
    title: 'Direct arXiv Access',
    description: 'Read PDFs and HTML versions directly on arXiv. No broken embeds, no workarounds.',
  },
  {
    icon: Zap,
    title: 'Fast & Lightweight',
    description: 'No bloat. No ads. No paywalls. Just a clean, fast interface built for reading.',
  },
];

const STATS = [
  { value: '2.5M+', label: 'arXiv Papers' },
  { value: '220M+', label: 'Total Papers' },
  { value: 'Free', label: 'Forever' },
];

const TESTIMONIAL_FIELDS = [
  'Machine Learning', 'Quantum Computing', 'Computer Vision',
  'Natural Language Processing', 'Robotics', 'Cryptography',
  'Astrophysics', 'Condensed Matter', 'High Energy Physics',
  'Mathematics', 'Statistics', 'Economics',
];

export default function Landing() {
  const navigate = useNavigate();

  return (
    <div className="min-h-screen bg-white dark:bg-surface-950">
      {/* Header */}
      <header className="border-b border-surface-100 dark:border-surface-900">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 flex items-center justify-between h-16">
          <div className="flex items-center gap-2">
            <BookOpen className="h-7 w-7 text-primary-600" />
            <span className="text-xl font-bold tracking-tight text-surface-900 dark:text-surface-100">Da Papers</span>
          </div>
          <button
            onClick={() => navigate('/login')}
            className="px-5 py-2 text-sm font-medium rounded-xl bg-primary-600 hover:bg-primary-700 text-white transition-colors"
          >
            Get Started Free
          </button>
        </div>
      </header>

      {/* Hero Section */}
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-br from-primary-50 via-white to-amber-50 dark:from-surface-950 dark:via-surface-950 dark:to-surface-900" />
        <div className="relative max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 pt-20 pb-24 sm:pt-28 sm:pb-32">
          <div className="max-w-3xl mx-auto text-center">
            <div className="inline-flex items-center gap-2 px-4 py-1.5 rounded-full bg-primary-100 dark:bg-primary-900/50 text-primary-700 dark:text-primary-300 text-sm font-medium mb-8">
              <GraduationCap className="h-4 w-4" />
              Built for researchers, by researchers
            </div>

            <h1 className="text-4xl sm:text-5xl lg:text-6xl font-bold text-surface-900 dark:text-surface-100 leading-tight tracking-tight">
              Your research papers,{' '}
              <span className="text-primary-600 dark:text-primary-400">organized</span>
            </h1>

            <p className="mt-6 text-lg sm:text-xl text-surface-600 dark:text-surface-400 leading-relaxed max-w-2xl mx-auto">
              Search millions of papers with citation counts, save what matters, and build your personal research library. 
              The clean, fast reading companion for PhDs, students, and professors.
            </p>

            <div className="mt-10">
              <button
                onClick={() => navigate('/login')}
                className="w-full sm:w-auto px-8 py-4 text-base font-semibold rounded-xl bg-primary-600 hover:bg-primary-700 text-white shadow-lg shadow-primary-600/25 hover:shadow-primary-600/40 transition-all"
              >
                Start Reading — It's Free
              </button>
            </div>
          </div>
        </div>
      </section>

      {/* Stats Bar */}
      <section className="border-y border-surface-200 dark:border-surface-800 bg-surface-50 dark:bg-surface-900">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          <div className="grid grid-cols-3 gap-8">
            {STATS.map((stat) => (
              <div key={stat.label} className="text-center">
                <p className="text-2xl sm:text-3xl font-bold text-surface-900 dark:text-surface-100">{stat.value}</p>
                <p className="text-sm text-surface-500 dark:text-surface-400 mt-1">{stat.label}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Fields Ticker */}
      <section className="py-10 overflow-hidden bg-white dark:bg-surface-950">
        <p className="text-center text-sm text-surface-400 dark:text-surface-500 mb-4 font-medium uppercase tracking-wider">
          Papers across every field
        </p>
        <div className="flex gap-3 animate-scroll">
          {[...TESTIMONIAL_FIELDS, ...TESTIMONIAL_FIELDS].map((field, i) => (
            <span
              key={i}
              className="flex-shrink-0 px-4 py-2 rounded-full bg-surface-100 dark:bg-surface-800 text-surface-600 dark:text-surface-400 text-sm font-medium whitespace-nowrap"
            >
              {field}
            </span>
          ))}
        </div>
      </section>

      {/* Features Grid */}
      <section className="py-20 sm:py-28 bg-white dark:bg-surface-950">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-2xl mx-auto mb-16">
            <h2 className="text-3xl sm:text-4xl font-bold text-surface-900 dark:text-surface-100">
              Everything you need to read smarter
            </h2>
            <p className="mt-4 text-lg text-surface-500 dark:text-surface-400">
              No more juggling tabs, losing references, or wading through clunky interfaces.
            </p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-8">
            {FEATURES.map((feature) => (
              <div
                key={feature.title}
                className="group p-6 rounded-2xl border border-surface-200 dark:border-surface-800 hover:border-primary-200 dark:hover:border-primary-800 hover:shadow-lg hover:shadow-primary-50 dark:hover:shadow-primary-950/50 transition-all"
              >
                <div className="w-12 h-12 rounded-xl bg-primary-50 dark:bg-primary-950 flex items-center justify-center mb-4 group-hover:bg-primary-100 dark:group-hover:bg-primary-900 transition-colors">
                  <feature.icon className="h-6 w-6 text-primary-600 dark:text-primary-400" />
                </div>
                <h3 className="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-2">
                  {feature.title}
                </h3>
                <p className="text-surface-500 dark:text-surface-400 leading-relaxed">
                  {feature.description}
                </p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Who It's For */}
      <section className="py-20 bg-surface-50 dark:bg-surface-900/50">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
          <h2 className="text-3xl sm:text-4xl font-bold text-center text-surface-900 dark:text-surface-100 mb-16">
            Built for the way you work
          </h2>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-10">
            <div className="text-center">
              <div className="w-16 h-16 rounded-2xl bg-blue-100 dark:bg-blue-950 flex items-center justify-center mx-auto mb-5">
                <GraduationCap className="h-8 w-8 text-blue-600 dark:text-blue-400" />
              </div>
              <h3 className="text-xl font-semibold text-surface-900 dark:text-surface-100 mb-3">PhD Students</h3>
              <p className="text-surface-500 dark:text-surface-400 leading-relaxed">
                Keep your literature review organized. Track what you've read, bookmark key papers, and add notes as you go.
              </p>
            </div>
            <div className="text-center">
              <div className="w-16 h-16 rounded-2xl bg-amber-100 dark:bg-amber-950 flex items-center justify-center mx-auto mb-5">
                <BookOpen className="h-8 w-8 text-amber-600 dark:text-amber-400" />
              </div>
              <h3 className="text-xl font-semibold text-surface-900 dark:text-surface-100 mb-3">Professors</h3>
              <p className="text-surface-500 dark:text-surface-400 leading-relaxed">
                Stay on top of the latest research. Quickly find highly-cited papers and manage your reading across projects.
              </p>
            </div>
            <div className="text-center">
              <div className="w-16 h-16 rounded-2xl bg-emerald-100 dark:bg-emerald-950 flex items-center justify-center mx-auto mb-5">
                <Globe className="h-8 w-8 text-emerald-600 dark:text-emerald-400" />
              </div>
              <h3 className="text-xl font-semibold text-surface-900 dark:text-surface-100 mb-3">Independent Researchers</h3>
              <p className="text-surface-500 dark:text-surface-400 leading-relaxed">
                Access the same papers as top institutions. No paywalls, no institutional login required. Just sign up and start.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* CTA Section */}
      <section className="py-20 sm:py-28 bg-white dark:bg-surface-950">
        <div className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 text-center">
          <h2 className="text-3xl sm:text-4xl font-bold text-surface-900 dark:text-surface-100">
            Ready to streamline your research?
          </h2>
          <p className="mt-4 text-lg text-surface-500 dark:text-surface-400">
            Join researchers who use Da Papers to find, read, and organize academic papers.
          </p>
          <div className="mt-10">
            <button
              onClick={() => navigate('/login')}
              className="w-full sm:w-auto px-8 py-4 text-base font-semibold rounded-xl bg-primary-600 hover:bg-primary-700 text-white shadow-lg shadow-primary-600/25 transition-all"
            >
              Get Started Free
            </button>
          </div>
          <p className="mt-4 text-sm text-surface-400">
            No credit card required. Takes 10 seconds.
          </p>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-surface-200 dark:border-surface-800 bg-surface-50 dark:bg-surface-900 py-8">
        <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex flex-col sm:flex-row items-center justify-between gap-4">
            <div className="flex items-center gap-2">
              <BookOpen className="h-5 w-5 text-primary-600" />
              <span className="font-semibold text-surface-900 dark:text-surface-100">Da Papers</span>
            </div>
            <p className="text-sm text-surface-400">
              Data from <a href="https://arxiv.org" target="_blank" rel="noopener noreferrer" className="text-primary-600 hover:underline">arXiv.org</a> &amp; <a href="https://openalex.org" target="_blank" rel="noopener noreferrer" className="text-primary-600 hover:underline">OpenAlex</a>.
            </p>
          </div>
        </div>
      </footer>
    </div>
  );
}
