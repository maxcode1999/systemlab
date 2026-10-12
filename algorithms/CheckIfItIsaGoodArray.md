```

class Solution {

public:

bool isGoodArray(vector& nums) {
        int g = nums[0];

        for(auto& num: nums){
                g = std::gcd(g,num);
        }
        if(g == 1)
                return true;
        return false;

}
};

```
