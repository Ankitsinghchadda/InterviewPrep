import { createBrowserRouter } from 'react-router-dom'
import { Layout } from '@/components/layout/Layout'
import { Landing } from '@/pages/Landing'
import { Home } from '@/pages/Home'
import { Topics } from '@/pages/Topics'
import { Questions } from '@/pages/Questions'
import { QuestionDetail } from '@/pages/QuestionDetail'
import { Interview } from '@/pages/Interview'
import { InterviewRunner } from '@/pages/InterviewRunner'
import { Onboarding } from '@/pages/Onboarding'
import { Login } from '@/pages/Login'
import { ProtectedRoute } from '@/auth/ProtectedRoute'

export const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout />,
    children: [
      { index: true, element: <Landing /> },
      { path: 'login', element: <Login /> },
      {
        element: <ProtectedRoute />,
        children: [
          { path: 'dashboard', element: <Home /> },
          { path: 'onboarding', element: <Onboarding /> },
          { path: 'topics', element: <Topics /> },
          { path: 'questions', element: <Questions /> },
          { path: 'questions/:id', element: <QuestionDetail /> },
          { path: 'interview', element: <Interview /> },
          { path: 'interview/:id', element: <InterviewRunner /> },
        ],
      },
    ],
  },
])
