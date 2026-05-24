import{a as l}from"./react-D7rv8Q1m.js";import{Q as v,n as N,m,y,H as F,z as R,l as E}from"./index-D-ej_2Ja.js";var C=class extends v{constructor(e,s){super(e,s)}bindMethods(){super.bindMethods(),this.fetchNextPage=this.fetchNextPage.bind(this),this.fetchPreviousPage=this.fetchPreviousPage.bind(this)}setOptions(e){e._type="infinite",super.setOptions(e)}getOptimisticResult(e){return e._type="infinite",super.getOptimisticResult(e)}fetchNextPage(e){return this.fetch({...e,meta:{fetchMore:{direction:"forward"}}})}fetchPreviousPage(e){return this.fetch({...e,meta:{fetchMore:{direction:"backward"}}})}createResult(e,s){var d,P;const{state:r}=e,t=super.createResult(e,s),{isFetching:i,isRefetching:n,isError:a,isRefetchError:u}=t,o=(P=(d=r.fetchMeta)==null?void 0:d.fetchMore)==null?void 0:P.direction,c=a&&o==="forward",f=i&&o==="forward",h=a&&o==="backward",g=i&&o==="backward";return{...t,fetchNextPage:this.fetchNextPage,fetchPreviousPage:this.fetchPreviousPage,hasNextPage:m(s,r.data),hasPreviousPage:N(s,r.data),isFetchNextPageError:c,isFetchingNextPage:f,isFetchPreviousPageError:h,isFetchingPreviousPage:g,isRefetchError:u&&!c&&!h,isRefetching:n&&!f&&!g}}};function M(e,s){return y(e,C)}const x=500;function b(e){var i;const s=new Set,r=[];for(const n of e)for(const a of n.edges)s.has(a.node.id)||(s.add(a.node.id),r.push(a.node));const t=((i=e.at(-1))==null?void 0:i.totalCount)??r.length;return{projects:r,totalCount:t,loadedCount:r.length,truncated:r.length>=x&&r.length<t}}function j(e,s){return!(!e.pageInfo.hasNextPage||s>=x)}const p=100,w=`
  query GetProjects($first: Int, $after: String) {
    projects(first: $first, after: $after) {
      totalCount
      pageInfo {
        hasNextPage
        endCursor
      }
      edges {
        cursor
        node {
          id
          projectId
          name
          description
          isActive
          team
          canManage
          stats {
            totalTestRuns
            successRate
            averageDuration
            lastRunTime
          }
        }
      }
    }
  }
`;function Q(){const e=F(),{data:s}=R(),r=(s==null?void 0:s.role)??"anonymous";l.useEffect(()=>{e.invalidateQueries({queryKey:["projects","paged"]})},[r,e]);const t=M({queryKey:["projects","paged",p,r],queryFn:async({pageParam:n})=>{const{projects:a}=await E(w,{first:p,after:n});return a},initialPageParam:void 0,getNextPageParam:(n,a)=>{const u=a.reduce((o,c)=>o+c.edges.length,0);if(j(n,u))return n.pageInfo.endCursor??void 0},staleTime:6e4});return l.useEffect(()=>{t.hasNextPage&&!t.isFetchingNextPage&&!t.isFetching&&t.fetchNextPage()},[t.hasNextPage,t.isFetchingNextPage,t.isFetching,t]),{data:t.data?b(t.data.pages):void 0,isLoading:t.isLoading,isFetchingMore:t.isFetchingNextPage,error:t.error??null}}export{Q as u};
//# sourceMappingURL=hooks-Ovx7giMg.js.map
