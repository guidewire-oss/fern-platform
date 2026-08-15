import{H as p,l as u,p as e,f as g,E as m,T as h}from"./index-8Y9R7xA4.js";import{a as f}from"./react-D7rv8Q1m.js";import{C as j}from"./Card-BqB8nOdF.js";import{S as y}from"./search-D6qVM-0m.js";const N=`
  query AllTags($first: Int) {
    tags(first: $first) {
      totalCount
      edges {
        node {
          id
          name
          category
          value
          description
        }
      }
    }
  }
`;function S(){const[o,x]=f.useState(""),{data:a,isLoading:l,error:n}=p({queryKey:["tags"],queryFn:()=>u(N,{first:200}),staleTime:6e4}),c=(a==null?void 0:a.tags.edges)??[],d=o?c.filter(({node:s})=>[s.name,s.category,s.value,s.description].filter(Boolean).some(t=>t.toLowerCase().includes(o.toLowerCase()))):c,r=new Map;for(const{node:s}of d){const t=s.category||"uncategorized";r.has(t)||r.set(t,[]),r.get(t).push(s)}return e.jsxs("div",{className:"space-y-6",children:[e.jsxs("header",{className:"flex items-baseline justify-between",children:[e.jsxs("div",{children:[e.jsx("h1",{className:"text-3xl font-semibold tracking-tight",children:"Tags"}),e.jsxs("p",{className:"mt-1 text-sm text-muted",children:[(a==null?void 0:a.tags.totalCount)??0," tags across ",r.size," categories"]})]}),e.jsxs("div",{className:"relative",children:[e.jsx(y,{className:"absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted"}),e.jsx("input",{type:"search",value:o,onChange:s=>x(s.target.value),placeholder:"Search tags…",className:"w-64 rounded-md border border-border bg-surface py-1.5 pl-7 pr-2 text-sm focus:border-primary focus:outline-none"})]})]}),l&&e.jsxs("div",{className:"flex items-center gap-2 text-muted",children:[e.jsx(g,{})," Loading tags…"]}),n&&e.jsx(m,{title:"Couldn't load tags",description:n.message}),!l&&d.length===0&&e.jsx(m,{title:"No tags match your search"}),e.jsx("div",{className:"grid gap-4 md:grid-cols-2 xl:grid-cols-3",children:Array.from(r.entries()).map(([s,t])=>e.jsxs(j,{children:[e.jsx("div",{className:"border-b border-border bg-gradient-to-r from-primary-soft to-transparent px-4 py-2.5",children:e.jsxs("div",{className:"flex items-center justify-between",children:[e.jsx("h3",{className:"text-sm font-semibold capitalize",children:s}),e.jsx("span",{className:"text-xs text-muted",children:t.length})]})}),e.jsx("ul",{className:"divide-y divide-border",children:t.map(i=>e.jsxs("li",{className:"flex items-center gap-2 px-4 py-2",children:[e.jsx(h,{className:"h-3.5 w-3.5 text-primary"}),e.jsx("span",{className:"font-mono text-xs text-foreground",children:i.name}),i.description&&e.jsx("span",{className:"ml-auto truncate text-[11px] text-muted",title:i.description,children:i.description})]},i.id))})]},s))})]})}export{S as default};
//# sourceMappingURL=TagsManagement-BgELXTfg.js.map
