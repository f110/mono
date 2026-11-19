import { useQuery } from '@connectrpc/connect-query'
import { ReviewService, type Review } from '../connect/codexel_pb.js'

// useReviews returns the reviews of the repository by the change ID. The reviews of each change are the newest first.
export const useReviews = (path: string | undefined): Map<string, Review[]> => {
  const { data } = useQuery(ReviewService.method.listReviews, { path }, { enabled: !!path })
  const reviews = new Map<string, Review[]>()
  for (const r of data?.reviews ?? []) {
    for (const id of r.changeIds) {
      reviews.set(id, [...(reviews.get(id) ?? []), r])
    }
  }
  return reviews
}
