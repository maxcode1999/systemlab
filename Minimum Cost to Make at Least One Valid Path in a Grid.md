1368. [Minimum Cost to Make at Least One Valid Path in a Grid](https://leetcode.com/problems/minimum-cost-to-make-at-least-one-valid-path-in-a-grid/description/)

```
class Solution {
public:

    void setadj(vector<vector<pair<int, int>>> &adjlist, int i, int j, int n, int m, int  nodeind, int freenextnode){
        int nextnode;
        if(j+1 < m){
            nextnode = i*m+j+1;
            adjlist[nodeind].push_back({nextnode, (nextnode == freenextnode)? 0: 1});
        }
        if(j-1 >= 0){
            nextnode = i*m+j-1;
            adjlist[nodeind].push_back({nextnode, (nextnode == freenextnode)? 0: 1});
        }
        if(i+1 < n){
            nextnode = (i+1)*m+j;
            adjlist[nodeind].push_back({nextnode, (nextnode == freenextnode)? 0: 1});
        }
        if(i-1 >= 0){
            nextnode = (i-1)*m+j;
            adjlist[nodeind].push_back({nextnode, (nextnode == freenextnode)? 0: 1});
        }
        return;
    }
    int minCost(vector<vector<int>>& grid) {
        vector<vector<pair<int, int>>> adjlist;
        int n=grid.size();int m=grid[0].size();
        adjlist.resize(m*n + 1);
        vector<bool>visited;
        visited.resize(m*n+1, false);  //reserve just allocates memory

        //create a graph out of this grid withe edge weights
        for(int i=0;i<n;i++)
            for(int j=0;j<m;j++){
                int nodeind = i*m+j;  //row_i * num of cols + col_j
                if(grid[i][j] == 1){
                    setadj(adjlist, i, j, n, m, nodeind, i*m+j+1);
                } else if(grid[i][j] == 2){
                    setadj(adjlist, i, j, n, m, nodeind, i*m+j-1);
                } else if(grid[i][j] == 3){
                    setadj(adjlist, i, j, n, m, nodeind, (i+1)*m+j);
                } else {
                    setadj(adjlist, i, j, n, m, nodeind, (i-1)*m+j);
                }
            }
        
        //run djistra from 0 till we get to last node
        int lastnode = n*m-1;
        //prriority queue with pair, greater implies min heap
        priority_queue<pair<int,int>, vector<pair<int, int>>, greater<pair<int,int>> > pq;
        pq.push({0,0});
        while(true){
            //get first node
            int curd = pq.top().first;
            int curn = pq.top().second;
            pq.pop();

            //same node can be pushed multiple times until we see it first time. so need to avoid here
            if(visited[curn] == true)
                continue;
            visited[curn] = true;

            if(curn == lastnode)
                return curd;
            
            //push neighbours with distance if not visited
            for(int i=0;i<adjlist[curn].size();i++){
                if(visited[adjlist[curn][i].first] == false)
                    pq.push({adjlist[curn][i].second+curd, adjlist[curn][i].first});
            }
        }
        return 0;
    }
};
```
