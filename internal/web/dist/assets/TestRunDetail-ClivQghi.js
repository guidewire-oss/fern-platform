import{D as $,G as M,l as F,p as e,f as E,E as o,L as y,C as T}from"./index-D-ej_2Ja.js";import{a as g}from"./react-D7rv8Q1m.js";import{C as x,b as N,a as m,c as k}from"./Card-BUhOv3y-.js";import{S as u}from"./StatusBadge-DL6-YJMj.js";import{e as w,c as C,d,b as i,T as R,a as n}from"./Table-D7qQb06r.js";import{f as h}from"./duration-BdrzOpKg.js";import{A as D}from"./arrow-left-CooTq-sb.js";import{C as I}from"./chevron-right-BtBm2Ro5.js";const L=`
  query GetTestRun($id: ID!) {
    testRun(id: $id) {
      id
      projectId
      runId
      branch
      commitSha
      status
      startTime
      endTime
      duration
      totalTests
      passedTests
      failedTests
      skippedTests
      environment
      metadata
      tags {
        id
        name
        category
        value
        color
      }
      suiteRuns {
        id
        suiteName
        status
        startTime
        endTime
        duration
        totalSpecs
        passedSpecs
        failedSpecs
        skippedSpecs
        tags {
          id
          name
          category
          value
          color
        }
        specRuns {
          id
          specName
          status
          startTime
          endTime
          duration
          errorMessage
          stackTrace
          retryCount
          isFlaky
          tags {
            id
            name
            category
            value
            color
          }
        }
      }
    }
  }
`;function A(s){return s==null?!1:Array.isArray(s)?s.length>0:typeof s=="object"?Object.keys(s).length>0:!1}function W(){const{runId:s}=$({from:"/test-runs/$runId"}),{data:a,isLoading:t,error:r}=M({queryKey:["test-run",s],queryFn:()=>F(L,{id:s})}),[v,b]=g.useState(null);if(t)return e.jsxs("div",{className:"flex items-center gap-2 text-muted",children:[e.jsx(E,{})," Loading run ",s,"…"]});if(r)return e.jsx(o,{title:"Couldn't load run",description:r.message});const l=a==null?void 0:a.testRun;if(!l)return e.jsx(o,{title:`Run ${s} not found`});const j=l.suiteRuns??[],S=v?j.find(f=>f.id===v)??null:null;return e.jsxs("div",{className:"space-y-4",children:[e.jsxs(y,{to:"/test-runs",className:"inline-flex items-center gap-1 text-sm text-muted hover:text-foreground",children:[e.jsx(D,{className:"h-3 w-3"})," All test runs"]}),e.jsx(q,{run:l}),A(l.metadata)&&e.jsx(G,{value:l.metadata}),j.length===0?e.jsx(o,{title:"No suite details available",description:"This run was recorded without suite information (or the seeder skipped suites)."}):S?e.jsx(H,{suite:S,onBack:()=>b(null)}):e.jsx(B,{suites:j,onSelect:f=>b(f)})]})}function q({run:s}){return e.jsxs("header",{className:"space-y-2",children:[e.jsxs("div",{className:"flex items-center gap-2",children:[e.jsx("h1",{className:"text-2xl font-semibold",children:s.runId}),e.jsx(u,{status:s.status})]}),e.jsxs("div",{className:"text-sm text-muted",children:[e.jsx(y,{to:"/projects/$projectId",params:{projectId:s.projectId},className:"hover:text-foreground",children:s.projectId})," ","· ",s.branch||"no branch"," · ",new Date(s.startTime).toLocaleString()]}),s.tags.length>0&&e.jsx("div",{className:"flex flex-wrap gap-1","data-testid":"run-tags",children:s.tags.map(a=>e.jsx(p,{tag:a},a.id))}),e.jsxs("div",{className:"grid grid-cols-2 gap-3 md:grid-cols-4",children:[e.jsx(c,{label:"Total",value:s.totalTests}),e.jsx(c,{label:"Passed",value:s.passedTests,tone:"text-green-700"}),e.jsx(c,{label:"Failed",value:s.failedTests,tone:"text-red-700"}),e.jsx(c,{label:"Skipped",value:s.skippedTests,tone:"text-amber-700"})]}),e.jsxs("div",{className:"text-xs text-muted",children:["Duration ",h(s.duration),s.environment?` · env ${s.environment}`:"",s.commitSha?` · commit ${s.commitSha.slice(0,8)}`:""]})]})}function B({suites:s,onSelect:a}){return e.jsxs(x,{children:[e.jsx(N,{children:e.jsxs(k,{children:["Test Suites (",s.length,")"]})}),e.jsx(m,{className:"overflow-x-auto p-0",children:e.jsxs(w,{children:[e.jsx(C,{children:e.jsxs(d,{children:[e.jsx(i,{children:"Suite Name"}),e.jsx(i,{className:"text-right",children:"Test Results"}),e.jsx(i,{children:"Status"}),e.jsx(i,{className:"text-right",children:"Duration"}),e.jsx(i,{children:"Tags"})]})}),e.jsx(R,{children:s.map(t=>e.jsxs(d,{role:"button",tabIndex:0,onClick:()=>a(t.id),onKeyDown:r=>{(r.key==="Enter"||r.key===" ")&&(r.preventDefault(),a(t.id))},className:"cursor-pointer hover:bg-surface-2","data-testid":"suite-row",children:[e.jsxs(n,{children:[e.jsx("div",{className:"font-medium",children:t.suiteName}),e.jsxs("div",{className:"text-[11px] text-muted",children:[t.totalSpecs," specs"]})]}),e.jsxs(n,{className:"text-right tabular-nums text-sm",children:[e.jsx("span",{className:"text-green-700",title:"passed",children:t.passedSpecs})," / ",e.jsx("span",{className:"text-red-700",title:"failed",children:t.failedSpecs})," / ",e.jsx("span",{className:"text-muted",title:"skipped",children:t.skippedSpecs})]}),e.jsx(n,{children:e.jsx(u,{status:t.status})}),e.jsx(n,{className:"text-right tabular-nums",children:h(t.duration)}),e.jsx(n,{children:t.tags.length>0?e.jsx("span",{className:"flex flex-wrap gap-1","data-testid":"suite-tags",children:t.tags.map(r=>e.jsx(p,{tag:r},r.id))}):e.jsx("span",{className:"text-muted",children:"—"})})]},t.id))})]})})]})}function H({suite:s,onBack:a}){return e.jsxs(x,{children:[e.jsxs(N,{className:"space-y-2",children:[e.jsxs("button",{type:"button",onClick:a,className:"inline-flex items-center gap-1 text-sm text-muted hover:text-foreground","data-testid":"back-to-suites",children:[e.jsx(D,{className:"h-3 w-3"})," All suites"]}),e.jsxs("div",{className:"flex items-center justify-between",children:[e.jsxs("div",{children:[e.jsxs(k,{children:["Test Specs — ",s.suiteName]}),e.jsxs("div",{className:"text-xs text-muted",children:[s.totalSpecs," specs (",s.passedSpecs," passed, ",s.failedSpecs," failed",s.skippedSpecs>0?`, ${s.skippedSpecs} skipped`:"",") ·"," ",h(s.duration)]})]}),e.jsx(u,{status:s.status})]}),s.tags.length>0&&e.jsx("div",{className:"flex flex-wrap gap-1","data-testid":"suite-tags",children:s.tags.map(t=>e.jsx(p,{tag:t},t.id))})]}),e.jsx(m,{className:"overflow-x-auto p-0",children:s.specRuns.length===0?e.jsx(o,{title:"No spec details available",description:"This suite was recorded without per-spec rows."}):e.jsxs(w,{children:[e.jsx(C,{children:e.jsxs(d,{children:[e.jsx(i,{children:"Test Name"}),e.jsx(i,{children:"Status"}),e.jsx(i,{className:"text-right",children:"Duration"}),e.jsx(i,{children:"Error Message"}),e.jsx(i,{children:"Tags"}),e.jsx(i,{children:"Started"})]})}),e.jsx(R,{children:s.specRuns.map(t=>e.jsx(O,{spec:t},t.id))})]})})]})}function O({spec:s}){const[a,t]=g.useState(!1);return e.jsxs(e.Fragment,{children:[e.jsxs(d,{"data-testid":"spec-row",children:[e.jsxs(n,{children:[e.jsx("div",{className:"font-medium",children:s.specName}),(s.isFlaky||s.retryCount>0)&&e.jsxs("div",{className:"text-[11px] text-amber-700",children:[s.isFlaky?"flaky":null,s.isFlaky&&s.retryCount>0?" · ":"",s.retryCount>0?e.jsxs("span",{"data-testid":"spec-retry-count",children:["retried ×",s.retryCount]}):null]})]}),e.jsx(n,{children:e.jsx(u,{status:s.status})}),e.jsx(n,{className:"text-right tabular-nums",children:h(s.duration)}),e.jsx(n,{children:s.errorMessage?e.jsxs("button",{type:"button",onClick:()=>t(r=>!r),className:"inline-flex max-w-[28rem] items-center gap-1 truncate rounded bg-red-50 px-1.5 py-0.5 text-left font-mono text-[11px] text-red-800 hover:bg-red-100",title:s.errorMessage,"aria-expanded":a,"data-testid":"spec-error-message",children:[a?e.jsx(T,{className:"h-3 w-3 shrink-0"}):e.jsx(I,{className:"h-3 w-3 shrink-0"}),e.jsx("span",{className:"truncate",children:s.errorMessage})]}):e.jsx("span",{className:"text-muted",children:"—"})}),e.jsx(n,{children:s.tags.length>0?e.jsx("span",{className:"flex flex-wrap gap-1","data-testid":"spec-tags",children:s.tags.map(r=>e.jsx(p,{tag:r},r.id))}):e.jsx("span",{className:"text-muted",children:"—"})}),e.jsx(n,{className:"text-xs text-muted",children:s.startTime?new Date(s.startTime).toLocaleString():"—"})]}),a&&(s.errorMessage||s.stackTrace)&&e.jsx(d,{children:e.jsx(n,{colSpan:6,children:e.jsx("pre",{className:"overflow-x-auto rounded bg-slate-50 p-2 font-mono text-[11px] text-slate-700","data-testid":"spec-stack-trace",children:s.stackTrace??s.errorMessage})})})]})}function p({tag:s}){const a=s.value?`${s.name}: ${s.value}`:s.name,t=s.color?{backgroundColor:`${s.color}22`,color:s.color}:void 0;return e.jsx("span",{className:"inline-flex items-center rounded-full border border-border bg-surface-2 px-2 py-0.5 text-[10px] font-medium text-muted",style:t,"data-testid":"tag-chip",children:a})}function G({value:s}){const[a,t]=g.useState(!1);return e.jsxs(x,{children:[e.jsx(N,{children:e.jsxs("button",{type:"button",onClick:()=>t(r=>!r),className:"flex items-center gap-1 text-sm font-medium text-foreground","aria-expanded":a,"data-testid":"metadata-toggle",children:[a?e.jsx(T,{className:"h-3 w-3"}):e.jsx(I,{className:"h-3 w-3"}),"Metadata"]})}),a&&e.jsx(m,{children:e.jsx("pre",{className:"overflow-x-auto rounded bg-slate-50 p-2 font-mono text-xs text-slate-700","data-testid":"metadata-content",children:JSON.stringify(s,null,2)})})]})}function c({label:s,value:a,tone:t}){return e.jsx(x,{children:e.jsxs(m,{children:[e.jsx("div",{className:"text-xs uppercase tracking-wider text-muted",children:s}),e.jsx("div",{className:`mt-1 text-xl font-semibold tabular-nums ${t??""}`,children:a})]})})}export{W as default,A as hasDisplayableMetadata};
//# sourceMappingURL=TestRunDetail-ClivQghi.js.map
