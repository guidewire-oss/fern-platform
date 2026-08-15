import{a as l}from"./react-D7rv8Q1m.js";import{Q as m,n as v,m as j,y as N,I as E,z as R,H as F,l as y}from"./index-8Y9R7xA4.js";var I=class extends m{constructor(e,r){super(e,r)}bindMethods(){super.bindMethods(),this.fetchNextPage=this.fetchNextPage.bind(this),this.fetchPreviousPage=this.fetchPreviousPage.bind(this)}setOptions(e){e._type="infinite",super.setOptions(e)}getOptimisticResult(e){return e._type="infinite",super.getOptimisticResult(e)}fetchNextPage(e){return this.fetch({...e,meta:{fetchMore:{direction:"forward"}}})}fetchPreviousPage(e){return this.fetch({...e,meta:{fetchMore:{direction:"backward"}}})}createResult(e,r){var d,P;const{state:s}=e,t=super.createResult(e,r),{isFetching:i,isRefetching:n,isError:a,isRefetchError:u}=t,o=(P=(d=s.fetchMeta)==null?void 0:d.fetchMore)==null?void 0:P.direction,c=a&&o==="forward",f=i&&o==="forward",g=a&&o==="backward",h=i&&o==="backward";return{...t,fetchNextPage:this.fetchNextPage,fetchPreviousPage:this.fetchPreviousPage,hasNextPage:j(r,s.data),hasPreviousPage:v(r,s.data),isFetchNextPageError:c,isFetchingNextPage:f,isFetchPreviousPageError:g,isFetchingPreviousPage:h,isRefetchError:u&&!c&&!g,isRefetching:n&&!f&&!h}}};function C(e,r){return N(e,I)}const x=500;function b(e){var i;const r=new Set,s=[];for(const n of e)for(const a of n.edges)r.has(a.node.id)||(r.add(a.node.id),s.push(a.node));const t=((i=e.at(-1))==null?void 0:i.totalCount)??s.length;return{projects:s,totalCount:t,loadedCount:s.length,truncated:s.length>=x&&s.length<t}}function q(e,r){return!(!e.pageInfo.hasNextPage||r>=x)}const p=100,M=`
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
`;function Q(){const e=E(),{data:r}=R(),s=(r==null?void 0:r.role)??"anonymous";l.useEffect(()=>{e.invalidateQueries({queryKey:["projects","paged"]})},[s,e]);const t=C({queryKey:["projects","paged",p,s],queryFn:async({pageParam:n})=>{const{projects:a}=await y(M,{first:p,after:n});return a},initialPageParam:void 0,getNextPageParam:(n,a)=>{const u=a.reduce((o,c)=>o+c.edges.length,0);if(q(n,u))return n.pageInfo.endCursor??void 0},staleTime:6e4});return l.useEffect(()=>{t.hasNextPage&&!t.isFetchingNextPage&&!t.isFetching&&t.fetchNextPage()},[t.hasNextPage,t.isFetchingNextPage,t.isFetching,t]),{data:t.data?b(t.data.pages):void 0,isLoading:t.isLoading,isFetchingMore:t.isFetchingNextPage,error:t.error??null}}const O=`
  query GetProjectByProjectId($projectId: String!) {
    projectByProjectId(projectId: $projectId) {
      projectId
      name
      team
    }
  }
`;function S(e){return F({queryKey:["project",e],queryFn:async()=>(await y(O,{projectId:e})).projectByProjectId,enabled:!!e,staleTime:5*6e4})}export{Q as a,S as u};
//# sourceMappingURL=hooks-C02LdgGA.js.map
