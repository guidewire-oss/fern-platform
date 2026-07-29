import{I as n,l as o}from"./index-GNsoRvIO.js";import{u as r}from"./Button-DYZuswws.js";const i=`
  mutation CreateProject($input: CreateProjectInput!) {
    createProject(input: $input) {
      id projectId name
    }
  }
`,c=`
  mutation UpdateProject($id: ID!, $input: UpdateProjectInput!) {
    updateProject(id: $id, input: $input) {
      id projectId name description team defaultBranch repository
    }
  }
`,a=`
  mutation DeleteProject($id: ID!) {
    deleteProject(id: $id)
  }
`;function d(){const e=n();return r({mutationFn:t=>o(i,{input:t}),onSuccess:()=>e.invalidateQueries({queryKey:["projects"]})})}function j(){const e=n();return r({mutationFn:({id:t,input:u})=>o(c,{id:t,input:u}),onSuccess:()=>e.invalidateQueries({queryKey:["projects"]})})}function m(){const e=n();return r({mutationFn:t=>o(a,{id:t}),onSuccess:()=>e.invalidateQueries({queryKey:["projects"]})})}export{m as a,j as b,d as u};
//# sourceMappingURL=mutations-DIoTt89z.js.map
