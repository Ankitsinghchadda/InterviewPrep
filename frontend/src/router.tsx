import { createBrowserRouter } from 'react-router-dom'
import { Layout } from '@/components/layout/Layout'
import { Landing } from '@/pages/Landing'
import { Contact } from '@/pages/Contact'
import { Home } from '@/pages/Home'
import { Topics } from '@/pages/Topics'
import { TopicDetail } from '@/pages/TopicDetail'
import { Questions } from '@/pages/Questions'
import { QuestionDetail } from '@/pages/QuestionDetail'
import { Library } from '@/pages/Library'
import { Interview } from '@/pages/Interview'
import { InterviewRunner } from '@/pages/InterviewRunner'
import { Onboarding } from '@/pages/Onboarding'
import { Login } from '@/pages/Login'
import { Pricing } from '@/pages/Pricing'
import { AccountBilling } from '@/pages/AccountBilling'
import { ProtectedRoute } from '@/auth/ProtectedRoute'

export const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout />,
    children: [
      { index: true, element: <Landing /> },
      { path: 'contact', element: <Contact /> },
      { path: 'login', element: <Login /> },
      { path: 'pricing', element: <Pricing /> },
      // Public SEO surfaces — viewable without login. Pages themselves gate
      // any auth-requiring features (e.g. "practice" / "see answer") behind
      // login. Sitemap lists every /topics/:slug and /questions/:slug.
      { path: 'topics/:slug', element: <TopicDetail /> },
      { path: 'questions/:id', element: <QuestionDetail /> },
      {
        element: <ProtectedRoute />,
        children: [
          { path: 'dashboard', element: <Home /> },
          { path: 'onboarding', element: <Onboarding /> },
          { path: 'topics', element: <Topics /> },
          { path: 'questions', element: <Questions /> },
          { path: 'library', element: <Library /> },
          { path: 'interview', element: <Interview /> },
          { path: 'interview/:id', element: <InterviewRunner /> },
          { path: 'account/billing', element: <AccountBilling /> },
        ],
      },
    ],
  },
])
