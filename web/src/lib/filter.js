// applyFilter applies the shared category + status filter to a job list.
// Views with additional local filters (e.g. TableView's search) compose
// on top of the returned array.

export function applyFilter(jobs, filter) {
  let result = jobs || [];
  if (filter.category) result = result.filter(j => j.category === filter.category);
  if (filter.status)   result = result.filter(j => j.status === filter.status);
  return result;
}
